// Postman 集合（Collection v2.x）导入导出：
// 导出：分组 → 文件夹（item），接口 → request（含请求头 / 查询参数 / JSON 体 / 响应样例）。
// 导入：文件夹 → 接口分组（同名同父复用），请求 → 接口（查询串与 JSON 体展平为请求参数）。
package service

import (
	"context"
	"encoding/json"
	"strings"

	shpostman "github.com/leihenshang/api-doc-go-share/postman"

	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
	"apidoc/internal/repository"
)

// ---- 导出 ----

// ExportPostman 导出 Postman Collection；groupID > 0 时只导出该分组及其子分组。
func (s *Service) ExportPostman(ctx context.Context, projectID, groupID uint64) (map[string]any, error) {
	p, err := s.repo.ProjectByID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	groups, err := s.repo.ListGroups(ctx, projectID, model.GroupTypeAPI)
	if err != nil {
		return nil, err
	}
	apis, err := s.repo.ListApisByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if groupID > 0 {
		keep := descendantGroupIDs(groups, groupID)
		filtered := make([]model.Group, 0, len(groups))
		for _, g := range groups {
			if _, ok := keep[g.ID]; ok {
				filtered = append(filtered, g)
			}
		}
		groups = filtered
		kept := make([]model.Api, 0, len(apis))
		for _, a := range apis {
			if _, ok := keep[a.GroupID]; ok {
				kept = append(kept, a)
			}
		}
		apis = kept
	}

	// 分组 → 文件夹；未分组或分组缺失的接口放顶层。
	known := map[uint64]struct{}{}
	for _, g := range groups {
		known[g.ID] = struct{}{}
	}
	col := &shpostman.Collection{Name: p.Title, Description: p.Description}
	ungrouped := make([]shpostman.Item, 0)
	for _, a := range apis {
		if _, ok := known[a.GroupID]; !ok {
			ungrouped = append(ungrouped, shpostman.Item{Name: a.ApiName, Req: postmanRequestOf(a)})
		}
	}
	for _, g := range groups {
		folder := shpostman.Item{Name: g.Title, Folder: true}
		for _, a := range apis {
			if a.GroupID == g.ID {
				folder.Items = append(folder.Items, shpostman.Item{Name: a.ApiName, Req: postmanRequestOf(a)})
			}
		}
		if len(folder.Items) == 0 {
			continue // 空分组不导出，避免文档里出现空文件夹
		}
		col.Items = append(col.Items, folder)
	}
	col.Items = append(col.Items, ungrouped...)
	return col.Encode(), nil
}

// postmanRequestOf 接口记录 → 中立请求（方法大写、空则 GET）。
func postmanRequestOf(a model.Api) *shpostman.Request {
	method := strings.ToUpper(strings.TrimSpace(a.HttpMethodType))
	if method == "" {
		method = "GET"
	}
	r := &shpostman.Request{
		Method:         method,
		URL:            a.URL,
		BodyRaw:        paramsBody(a.HttpRequestParams),
		Description:    strings.TrimSpace(a.Description),
		ResponseSample: strings.TrimSpace(a.HttpReturnSample),
	}
	for _, h := range parseRows(a.HttpRequestHeader) {
		name, _ := h["name"].(string)
		value, _ := h["value"].(string)
		if name == "" {
			continue
		}
		r.Headers = append(r.Headers, shpostman.Header{Name: name, Value: value})
	}
	return r
}

// descendantGroupIDs 返回分组及其全部后代 id（含自身）。
func descendantGroupIDs(groups []model.Group, root uint64) map[uint64]struct{} {
	out := map[uint64]struct{}{root: {}}
	for changed := true; changed; {
		changed = false
		for _, g := range groups {
			if _, ok := out[g.ID]; ok {
				continue
			}
			if _, parent := out[g.PID]; parent {
				out[g.ID] = struct{}{}
				changed = true
			}
		}
	}
	return out
}

// ---- 导入 ----

// ImportPostman 导入 Postman 集合（JSON 文本）。
func (s *Service) ImportPostman(ctx context.Context, userID uint64, typ int8, projectID uint64, strategy string, body []byte) (ImportResult, error) {
	if err := s.ensureProjectPermission(ctx, userID, typ, projectID, model.PermCreate); err != nil {
		return ImportResult{}, err
	}
	col, err := shpostman.Decode(body)
	if err != nil {
		return ImportResult{}, newBiz(errcode.CodeParamError, "Postman 集合解析失败：%v", err)
	}
	if len(col.Items) == 0 {
		return ImportResult{}, newBiz(errcode.CodeParamError, "集合里没有请求（item 为空），请确认导出的是 Collection v2.x")
	}
	var res ImportResult
	// 解析已在上方完成；清空（覆盖策略）与写入同事务，失败即整体回滚。
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
		ensure := s.postmanGroupEnsurer(sc, projectID, &groups)

		var walk func(items []shpostman.Item, parent uint64) error
		walk = func(items []shpostman.Item, parent uint64) error {
			for _, it := range items {
				if it.Folder {
					name := strings.TrimSpace(it.Name)
					if name == "" {
						name = "未命名目录"
					}
					gid, err := ensure(parent, name)
					if err != nil {
						return err
					}
					if err := walk(it.Items, gid); err != nil {
						return err
					}
					continue
				}
				if it.Req == nil {
					continue
				}
				a, err := buildPostmanApi(projectID, parent, it)
				if err != nil {
					res.addFailure(it.Name + "：" + err.Error())
					continue
				}
				if a == nil {
					res.addFailure(it.Name + "：缺少 method 或 url")
					continue
				}
				key := apiUniqueKey(a.HttpMethodType, a.URL)
				if _, dup := keys[key]; dup {
					res.Skipped++
					continue
				}
				if err := tx.CreateApi(ctx, a); err != nil {
					return err
				}
				keys[key] = struct{}{}
				res.Imported++
			}
			return nil
		}
		return walk(col.Items, 0)
	})
	if err != nil {
		return res, err
	}
	s.recordOp(ctx, projectID, userID, projectID, model.LogObjectAPI, "import",
		"导入 Postman 接口 "+itoa(res.Imported)+" 条（跳过 "+itoa(res.Skipped)+"，失败 "+itoa(res.Failed)+"）")
	return res, nil
}

// postmanGroupEnsurer 惰性建分组：同名同父直接复用（与 Bruno 导入一致）。
func (s *Service) postmanGroupEnsurer(sc importScope, projectID uint64, groups *[]model.Group) func(parent uint64, name string) (uint64, error) {
	return func(parent uint64, name string) (uint64, error) {
		for _, g := range *groups {
			if g.PID == parent && g.Title == name {
				return g.ID, nil
			}
		}
		g := &model.Group{ProjectID: projectID, PID: parent, Title: name, Type: model.GroupTypeAPI}
		if err := sc.repo.CreateGroup(sc.ctx, g); err != nil {
			return 0, err
		}
		*groups = append(*groups, *g)
		return g.ID, nil
	}
}

// buildPostmanApi 把中立请求转成接口记录；method/url 缺失时返回 (nil, nil)。
func buildPostmanApi(projectID, groupID uint64, item shpostman.Item) (*model.Api, error) {
	req := item.Req
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	raw := req.URL
	if method == "" || raw == "" {
		return nil, nil
	}
	a := &model.Api{
		ProjectID:      projectID,
		GroupID:        groupID,
		ApiName:        strings.TrimSpace(item.Name),
		HttpMethodType: method,
		URL:            raw,
		ProtocolType:   "http",
		Source:         SourcePostman,
	}
	if a.ApiName == "" {
		a.ApiName = method + " " + raw
	}
	params := make([]apiParamRow, 0)
	for _, kv := range shpostman.QueryPairs(raw) {
		params = append(params, apiParamRow{FieldName: kv[0], Type: "string", Example: kv[1]})
	}
	if strings.TrimSpace(req.BodyRaw) != "" {
		var v any
		if err := json.Unmarshal([]byte(req.BodyRaw), &v); err != nil {
			return nil, newBiz(errcode.CodeParamError, "请求体 JSON 解析失败：%v", err)
		}
		params = append(params, flattenJSON("", v, nil)...)
	}
	if len(params) > 0 {
		if b, err := json.Marshal(params); err == nil {
			a.HttpRequestParams = string(b)
		}
	}
	headers := make([]apiHeaderRow, 0)
	for _, h := range req.Headers {
		headers = append(headers, apiHeaderRow{Name: h.Name, Value: h.Value})
	}
	if len(headers) > 0 {
		if b, err := json.Marshal(headers); err == nil {
			a.HttpRequestHeader = string(b)
		}
	}
	return a, nil
}
