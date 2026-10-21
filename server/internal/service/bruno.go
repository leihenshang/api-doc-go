package service

// Bruno 集合导入：解析 Bruno（OpenCollection 1.x）目录内的 yml 文件并落库。
// 目录（folder.yml）映射为接口分组（支持多级），请求文件映射为接口；
// 查询参数与 JSON 请求体展平为请求参数，docs（Markdown）存入接口说明。

import (
	"context"
	"encoding/json"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/leihenshang/api-doc-go-share/collection"

	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
	"apidoc/internal/repository"
)

// apiHeaderRow / apiParamRow 与前端 ApiHeader / ApiParam 的 JSON 契约对齐。
type apiHeaderRow struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

type apiParamRow struct {
	FieldName   string `json:"fieldName"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
	Example     string `json:"example"`
}

// ImportBruno 导入 Bruno 集合目录。files 为集合内的 yml 文本文件（相对路径 + 内容），
// 目录（folder.yml）映射为分组，请求文件映射为接口，写入来源 bruno-import。
func (s *Service) ImportBruno(ctx context.Context, userID uint64, typ int8, projectID uint64, strategy string, files []dto.BrunoFile) (ImportResult, error) {
	if err := s.ensureProjectPermission(ctx, userID, typ, projectID, model.PermCreate); err != nil {
		return ImportResult{}, err
	}
	if len(files) == 0 {
		return ImportResult{}, newBiz(errcode.CodeParamError, "请选择 Bruno 集合目录")
	}
	gids, reqs, err := splitBrunoFiles(files)
	if err != nil {
		return ImportResult{}, err
	}
	var res ImportResult
	// 文件已解析完毕；清空（覆盖策略）与写入同事务，失败即整体回滚。
	err = s.repo.Transaction(ctx, func(tx *repository.Repository) error {
		sc := importScope{ctx: ctx, repo: tx}
		keys, err := s.prepareImport(sc, projectID, strategy)
		if err != nil {
			return err
		}
		groups, err := tx.ListGroups(ctx, projectID, model.GroupTypeAPI)
		if err != nil {
			return err
		}
		ensure := s.brunoGroupEnsurer(sc, projectID, &groups, gids)

		for _, f := range reqs {
			req, err := collection.DecodeRequest([]byte(f.Content))
			if err != nil {
				res.addFailure(f.Path + "：YAML 解析失败")
				continue
			}
			if req.Info.Type != "http" {
				// 环境变量、脚本等非请求文件，按设计跳过，不计入失败
				continue
			}
			if strings.TrimSpace(req.HTTP.URL) == "" || strings.TrimSpace(req.HTTP.Method) == "" {
				res.addFailure(f.Path + "：缺少 method 或 url")
				continue
			}
			gid, err := ensure(path.Dir(f.Path))
			if err != nil {
				return err
			}
			key := apiUniqueKey(req.HTTP.Method, req.HTTP.URL)
			if _, dup := keys[key]; dup {
				res.Skipped++
				continue
			}
			a, err := s.buildBrunoApi(projectID, gid, req)
			if err != nil {
				res.addFailure(f.Path + "：" + err.Error())
				continue
			}
			if err := tx.CreateApi(ctx, a); err != nil {
				return err
			}
			keys[key] = struct{}{}
			res.Imported++
		}
		return nil
	})
	if err != nil {
		return res, err
	}
	s.recordOp(ctx, projectID, userID, projectID, model.LogObjectAPI, "import",
		"导入 Bruno 接口 "+itoa(res.Imported)+" 条（跳过 "+itoa(res.Skipped)+"，失败 "+itoa(res.Failed)+"）")
	return res, nil
}

// splitBrunoFiles 把集合文件拆成「目录 -> folder.yml 内容」与「请求文件」两类。
func splitBrunoFiles(files []dto.BrunoFile) (map[string]string, []dto.BrunoFile, error) {
	gids := map[string]string{}
	var reqs []dto.BrunoFile
	for _, f := range files {
		p := path.Clean(strings.ReplaceAll(f.Path, "\\", "/"))
		base := path.Base(p)
		switch {
		case base == "folder.yml" || base == "folder.yaml":
			gids[path.Dir(p)] = f.Content
		case base == "opencollection.yml" || base == "bruno.json" || strings.HasPrefix(base, "."):
			// 集合描述 / 忽略项，跳过
		case strings.HasSuffix(base, ".yml") || strings.HasSuffix(base, ".yaml") || strings.HasSuffix(base, ".bru"):
			reqs = append(reqs, dto.BrunoFile{Path: p, Content: f.Content})
		}
	}
	return gids, reqs, nil
}

// maxImportDepth 导入时的目录/文件夹嵌套上限（审核 H-5）：递归深度由请求体控制，
// 无上限时 Go 栈溢出是 fatal error（gin.Recovery 捕获不到）→ 整个进程崩。
const maxImportDepth = 32

// brunoGroupEnsurer 返回「目录 -> 分组 id」的惰性建组器：父目录先建，同名同级分组复用。
func (s *Service) brunoGroupEnsurer(sc importScope, projectID uint64, groups *[]model.Group, folderYAML map[string]string) func(dir string) (uint64, error) {
	gidByDir := map[string]uint64{".": 0, "": 0}
	var ensureAt func(dir string, depth int) (uint64, error)
	ensureAt = func(dir string, depth int) (uint64, error) {
		if depth > maxImportDepth {
			return 0, newBiz(errcode.CodeParamError, "目录层级过深（最多 %d 层）", maxImportDepth)
		}
		if gid, ok := gidByDir[dir]; ok {
			return gid, nil
		}
		parent, err := ensureAt(path.Dir(dir), depth+1)
		if err != nil {
			return 0, err
		}
		name := path.Base(dir)
		if raw, ok := folderYAML[dir]; ok {
			if fd, ferr := collection.DecodeFolder([]byte(raw)); ferr == nil && strings.TrimSpace(fd.Info.Name) != "" {
				name = strings.TrimSpace(fd.Info.Name)
			}
		}
		for _, g := range *groups {
			if g.PID == parent && g.Title == name {
				gidByDir[dir] = g.ID
				return g.ID, nil
			}
		}
		g := &model.Group{ProjectID: projectID, PID: parent, Title: name, Type: model.GroupTypeAPI}
		if err := sc.repo.CreateGroup(sc.ctx, g); err != nil {
			return 0, err
		}
		*groups = append(*groups, *g)
		gidByDir[dir] = g.ID
		return g.ID, nil
	}
	return func(dir string) (uint64, error) { return ensureAt(dir, 0) }
}

// buildBrunoApi 把请求文件转为接口记录：方法/路径/说明 + 请求头 + 查询参数与 JSON 请求体展平。
func (s *Service) buildBrunoApi(projectID, groupID uint64, req *collection.RequestFile) (*model.Api, error) {
	a := &model.Api{
		ProjectID:      projectID,
		GroupID:        groupID,
		ApiName:        strings.TrimSpace(req.Info.Name),
		HttpMethodType: strings.ToUpper(strings.TrimSpace(req.HTTP.Method)),
		URL:            strings.TrimSpace(req.HTTP.URL),
		ProtocolType:   "http",
		Description:    req.Docs,
		Source:         SourceBruno,
	}
	// 审核 H-8：导入的接口必须有 ext_uid，否则同步 appendItem 会直接丢弃（客户端永远拉不到）
	a.ExtUID = ensureExtUID("")
	if a.ApiName == "" {
		a.ApiName = a.HttpMethodType + " " + a.URL
	}

	params := make([]apiParamRow, 0, len(req.HTTP.Params))
	for _, p := range req.HTTP.Params {
		params = append(params, apiParamRow{FieldName: p.Name, Type: "string", Example: p.Value})
	}
	if raw := strings.TrimSpace(bodyContent(req.HTTP.Body)); strings.EqualFold(req.HTTP.Body.Type, "json") && raw != "" {
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			return nil, newBiz(errcode.CodeParamError, "请求体 JSON 解析失败（%s）：%v", a.ApiName, err)
		}
		params = append(params, flattenJSON("", v, nil)...)
	}
	if len(params) > 0 {
		if b, err := json.Marshal(params); err == nil {
			a.HttpRequestParams = string(b)
		}
	}
	if len(req.HTTP.Headers) > 0 {
		headers := make([]apiHeaderRow, 0, len(req.HTTP.Headers))
		for _, h := range req.HTTP.Headers {
			headers = append(headers, apiHeaderRow{Name: h.Name, Value: h.Value})
		}
		if b, err := json.Marshal(headers); err == nil {
			a.HttpRequestHeader = string(b)
		}
	}
	return a, nil
}

// bodyContent 取请求体原文：Bruno 写在 data，客户端写的集合写在 raw。
func bodyContent(b collection.Body) string {
	if strings.TrimSpace(b.Data) != "" {
		return b.Data
	}
	return b.Raw
}

// flattenJSON 把 JSON 值展平为 a.b[] 形式的参数行（对象逐层展开，数组按首元素推断结构）。
func flattenJSON(prefix string, v any, out []apiParamRow) []apiParamRow {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			p := k
			if prefix != "" {
				p = prefix + "." + k
			}
			out = append(out, apiParamRow{FieldName: p, Type: jsonTypeName(t[k]), Example: jsonExample(t[k])})
			out = flattenJSON(p, t[k], out)
		}
	case []any:
		if prefix == "" || len(t) == 0 {
			return out
		}
		if m, ok := t[0].(map[string]any); ok {
			out = flattenJSON(prefix+"[]", m, out)
		}
	}
	return out
}

// jsonTypeName 返回 JSON 值的类型名（整数与浮点区分）。
func jsonTypeName(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case float64:
		if t == float64(int64(t)) {
			return "integer"
		}
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return "string"
}

// jsonExample 取标量示例值（对象/数组不填）。
func jsonExample(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	}
	return ""
}
