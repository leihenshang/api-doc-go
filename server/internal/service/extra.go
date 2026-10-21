package service

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	shopenapi "github.com/leihenshang/api-doc-go-share/openapi"

	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
	"apidoc/internal/repository"
)

// importScope 把「上下文 + 写句柄」捆在一起传给导入辅助函数。
//
// 覆盖导入是「先删旧数据、再写新数据」的两段写：必须落在同一个事务里，
// 否则中途失败会留下一个被清空且只导入了一半的项目。所以这些辅助函数
// 不能再直接用服务上的 s.repo（那是事务外的句柄），必须接收事务句柄。
type importScope struct {
	ctx  context.Context
	repo *repository.Repository
}

// SaveFile 记录已上传文件。
func (s *Service) SaveFile(ctx context.Context, title, path string) (*model.FileRecord, error) {
	f := &model.FileRecord{Title: title, Path: path}
	if err := s.repo.CreateFile(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

// SaveFileWithHash 内容寻址保存（S13）：hash 为 sha256 hex。
func (s *Service) SaveFileWithHash(ctx context.Context, title, path, hash, mime string, size int64) (*model.FileRecord, error) {
	f := &model.FileRecord{Title: title, Path: path, Hash: hash, Mime: mime, Size: size}
	if err := s.repo.CreateFile(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

// GetFileByHash 按内容 hash 取文件记录（离线文档图片映射，R9）。
func (s *Service) GetFileByHash(ctx context.Context, hash string) (*model.FileRecord, error) {
	return s.repo.GetFileByHash(ctx, hash)
}

// ---- 接口导入（OpenAPI / Bruno） ----

// 导入来源标记，写入 api.source；空表示手工创建。
const (
	SourceOpenAPI = "openapi-import"
	SourceBruno   = "bruno-import"
	SourcePostman = "postman-import"
)

// 导入策略：覆盖=清空项目现有接口与接口目录后整体导入；追加=保留现状只新增（同方法+路径跳过）。
const (
	StrategyOverwrite = "overwrite"
	StrategyAppend    = "append"
)

// ImportResult 导入结果：imported 新增条数、skipped 因已存在跳过条数、failed 解析失败条数。
// Failures 给出失败明细（最多 10 条，供前端展示「状态说明」）。
type ImportResult struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Failed   int      `json:"failed"`
	Failures []string `json:"failures,omitempty"`
}

// addFailure 记录一条失败明细（明细最多保留 10 条，避免响应过大）。
func (r *ImportResult) addFailure(msg string) {
	r.Failed++
	if len(r.Failures) < 10 {
		r.Failures = append(r.Failures, msg)
	}
}

// normalizeStrategy 归一化导入策略，空值与未知值按「只新增」处理。
func normalizeStrategy(s string) string {
	if s == StrategyOverwrite {
		return StrategyOverwrite
	}
	return StrategyAppend
}

// apiUniqueKey 接口的去重键：大写方法 + 路径。
func apiUniqueKey(method, url string) string {
	return strings.ToUpper(strings.TrimSpace(method)) + " " + strings.TrimSpace(url)
}

// prepareImport 按策略准备项目：覆盖时清空现有接口与接口目录（结果集为纯导入内容），
// 追加时返回现存接口去重键集合（同键跳过）。
//
// 覆盖分支是破坏性写入，调用方必须保证它在事务内、且解析已经成功。
func (s *Service) prepareImport(sc importScope, projectID uint64, strategy string) (map[string]struct{}, error) {
	apis, err := sc.repo.ListApisByProject(sc.ctx, projectID)
	if err != nil {
		return nil, err
	}
	if normalizeStrategy(strategy) == StrategyOverwrite {
		if err := sc.repo.SoftDeleteApisByProject(sc.ctx, projectID); err != nil {
			return nil, err
		}
		if err := sc.repo.SoftDeleteGroupsByProject(sc.ctx, projectID, model.GroupTypeAPI); err != nil {
			return nil, err
		}
		return map[string]struct{}{}, nil
	}
	keys := make(map[string]struct{}, len(apis))
	for _, a := range apis {
		keys[apiUniqueKey(a.HttpMethodType, a.URL)] = struct{}{}
	}
	return keys, nil
}

// ImportOpenAPI 解析标准 OpenAPI(JSON) 并转为接口记录落库到指定项目+分组。
// 每个 (path, method) 生成一条接口记录；tag 映射为二级分组；写入来源 openapi-import。
func (s *Service) ImportOpenAPI(ctx context.Context, userID uint64, typ int8, projectID, groupID uint64, strategy string, body []byte) (ImportResult, error) {
	if err := s.ensureProjectPermission(ctx, userID, typ, projectID, model.PermCreate); err != nil {
		return ImportResult{}, err
	}
	// 先解析再动数据：覆盖策略会清空项目现有接口，若先把数据删了再发现
	// 文档解析不了，用户就白丢了一整个项目的内容。
	doc, err := shopenapi.Decode(body)
	if err != nil {
		return ImportResult{}, newBiz(errcode.CodeParamError, "OpenAPI 解析失败：%v", err)
	}
	var res ImportResult
	// 清空与导入同事务：任一硬错误（库不可用等）回滚，旧数据原样保留。
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
		for _, op := range doc.Ops {
			method := upperMethod(op.Method)
			if _, dup := keys[apiUniqueKey(method, op.Path)]; dup {
				res.Skipped++
				continue
			}
			target := s.resolveGroup(sc, projectID, groupID, op.Tags, &groups)
			a := &model.Api{
				ProjectID: projectID, GroupID: target,
				ApiName:        firstNonEmpty(op.Summary, op.OperationID, method+" "+op.Path),
				HttpMethodType: method, URL: op.Path, ProtocolType: "http",
				Source: SourceOpenAPI,
			}
			// 审核 H-8：导入的接口必须有 ext_uid，否则同步 appendItem 会直接丢弃（客户端永远拉不到）
			a.ExtUID = ensureExtUID("")
			// 参数 / 请求头 / 响应样例此前完全没写：导入只落名称、方法、URL，
			// 用户看到「导入成功」但打开接口三栏全空（UI 走查 P0-2）。
			applyOperationToApi(a, op)
			if err := tx.CreateApi(ctx, a); err != nil {
				return err
			}
			keys[apiUniqueKey(method, op.Path)] = struct{}{}
			res.Imported++
		}
		return nil
	})
	if err != nil {
		return res, err
	}
	s.recordOp(ctx, projectID, userID, projectID, model.LogObjectAPI, "import",
		"导入 OpenAPI 接口 "+itoa(res.Imported)+" 条（跳过 "+itoa(res.Skipped)+"）")
	return res, nil
}

// resolveGroup 依据 tag 找到或创建二级分组，返回其 id；空白 tag 归入默认分组，避免生成无标题的空分组。
//
// existing 必须传指针：新建分组要写回调用方的切片，否则同一份文档里
// 重复出现的同名 tag 每遇到一次就再建一个同名分组（按值传递的 append
// 永远不会被调用方看到）。
func (s *Service) resolveGroup(sc importScope, projectID, defaultGroup uint64, tags []string, existing *[]model.Group) uint64 {
	if len(tags) == 0 {
		return defaultGroup
	}
	tag := strings.TrimSpace(tags[0])
	if tag == "" {
		return defaultGroup
	}
	for _, g := range *existing {
		if g.Title == tag {
			return g.ID
		}
	}
	g := &model.Group{ProjectID: projectID, PID: defaultGroup, Title: tag, Type: model.GroupTypeAPI}
	if err := sc.repo.CreateGroup(sc.ctx, g); err == nil {
		*existing = append(*existing, *g)
		return g.ID
	}
	return defaultGroup
}

// firstNonEmpty 返回首个非空字符串。
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// upperMethod 归一化 HTTP 方法为大写。
func upperMethod(m string) string {
	b := []byte(m)
	for i := range b {
		if b[i] >= 'a' && b[i] <= 'z' {
			b[i] -= 32
		}
	}
	return string(b)
}

// itoa 简易整数转字符串。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// applyOperationToApi 把 OpenAPI 的 operation 映射到接口记录的文本列。
//
// 列格式必须与 Web 端 / 客户端既有约定一致（改这里要同步 utils/api-params.ts）：
//   - http_request_header：apiHeaderRow[] -> [{name,value,description}]
//   - http_request_params / http_return_params：apiParamRow[] -> [{fieldName,type,required,description,example}]
//   - http_return_sample：成功响应的 JSON 原文
//
// OpenAPI 的 path / query / cookie 参数统一落到「请求参数」（前端只有一张参数表），
// header 参数单独落到「请求头部」；返回参数没有 schema 时按 success example 的顶层字段推导，
// 至少让用户看到字段骨架而不是三个「暂无」。
func applyOperationToApi(a *model.Api, op shopenapi.Operation) {
	a.Description = op.Description

	var headers []apiHeaderRow
	var params []apiParamRow
	for _, p := range op.Params {
		if strings.EqualFold(p.In, "header") {
			headers = append(headers, apiHeaderRow{Name: p.Name, Value: p.Example, Description: p.Description})
			continue
		}
		params = append(params, apiParamRow{
			FieldName: p.Name, Required: p.Required, Example: p.Example, Description: p.Description,
		})
	}
	if len(headers) > 0 {
		if b, err := json.Marshal(headers); err == nil {
			a.HttpRequestHeader = string(b)
		}
	}
	if len(params) > 0 {
		if b, err := json.Marshal(params); err == nil {
			a.HttpRequestParams = string(b)
		}
	}

	// 成功样例：取第一个 2xx 且有 example 的响应（按状态码取最小，保证多次导入结果一致）。
	var success *shopenapi.Response
	for i := range op.Responses {
		r := &op.Responses[i]
		if !strings.HasPrefix(r.Code, "2") || r.ExampleJSON == "" {
			continue
		}
		if success == nil || r.Code < success.Code {
			success = r
		}
	}
	if success == nil {
		return
	}
	a.HttpReturnSample = success.ExampleJSON
	if rows := inferReturnParams(success.ExampleJSON); len(rows) > 0 {
		if b, err := json.Marshal(rows); err == nil {
			a.HttpReturnParams = string(b)
			a.HttpReturnType = "object"
		}
	}
}

// inferReturnParams 从响应样例的顶层字段推导返回参数表。
//
// 只取一层：嵌套对象/数组保留类型、example 给原文，够用且不会把表格撑爆。
// 字段顺序按名称排序——map 遍历无序，不排序会让同一份 spec 两次导入得到不同的表。
func inferReturnParams(exampleJSON string) []apiParamRow {
	var root map[string]any
	if json.Unmarshal([]byte(exampleJSON), &root) != nil {
		return nil
	}
	names := make([]string, 0, len(root))
	for k := range root {
		names = append(names, k)
	}
	sort.Strings(names)

	out := make([]apiParamRow, 0, len(names))
	for _, k := range names {
		out = append(out, apiParamRow{
			FieldName: k,
			Type:      jsonTypeName(root[k]),
			Example:   jsonExampleOrRaw(root[k]),
		})
	}
	return out
}

// jsonExampleOrRaw 与 bruno 的 jsonExample 同口径，额外把对象/数组按 JSON 文本回填，
// 否则嵌套字段的示例会是空串（前端那一列就白放着）。
func jsonExampleOrRaw(v any) string {
	switch v.(type) {
	case map[string]any, []any:
		if b, err := json.Marshal(v); err == nil {
			return string(b)
		}
		return ""
	default:
		return jsonExample(v)
	}
}
