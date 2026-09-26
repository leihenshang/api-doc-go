// Postman 集合（Collection v2.x）导入导出：
// 导出：分组 → 文件夹（item），接口 → request（含请求头 / 查询参数 / JSON 体 / 响应样例）。
// 导入：文件夹 → 接口分组（同名同父复用），请求 → 接口（查询串与 JSON 体展平为请求参数）。
package service

import (
	"context"
	"encoding/json"
	"strings"

	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
)

// postmanSchema Postman Collection v2.1 的 schema 标识。
const postmanSchema = "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"

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
	items := make([]map[string]any, 0, len(groups)+len(apis))
	ungrouped := make([]map[string]any, 0)
	for _, a := range apis {
		if _, ok := known[a.GroupID]; !ok {
			ungrouped = append(ungrouped, postmanRequestItem(a))
		}
	}
	for _, g := range groups {
		children := make([]map[string]any, 0)
		for _, a := range apis {
			if a.GroupID == g.ID {
				children = append(children, postmanRequestItem(a))
			}
		}
		if len(children) == 0 {
			continue
		}
		items = append(items, map[string]any{"name": g.Title, "item": children})
	}
	items = append(items, ungrouped...)
	if items == nil {
		items = []map[string]any{}
	}
	return map[string]any{
		"info": map[string]any{
			"name":        p.Title,
			"description": p.Description,
			"schema":      postmanSchema,
		},
		"item": items,
	}, nil
}

// postmanRequestItem 单个接口 → Postman item。
func postmanRequestItem(a model.Api) map[string]any {
	headers := make([]map[string]any, 0)
	for _, h := range parseRows(a.HttpRequestHeader) {
		name, _ := h["name"].(string)
		value, _ := h["value"].(string)
		if name == "" {
			continue
		}
		headers = append(headers, map[string]any{"key": name, "value": value, "type": "text"})
	}
	method := strings.ToUpper(strings.TrimSpace(a.HttpMethodType))
	if method == "" {
		method = "GET"
	}
	req := map[string]any{
		"method": method,
		"header": headers,
		"url":    a.URL,
	}
	if body := paramsBody(a.HttpRequestParams); body != "" {
		req["body"] = map[string]any{
			"mode":    "raw",
			"raw":     body,
			"options": map[string]any{"raw": map[string]any{"language": "json"}},
		}
	}
	if desc := strings.TrimSpace(a.Description); desc != "" {
		req["description"] = desc
	}
	item := map[string]any{"name": a.ApiName, "request": req}
	if sample := strings.TrimSpace(a.HttpReturnSample); sample != "" {
		item["response"] = []map[string]any{{
			"name":                     "成功响应",
			"status":                   "OK",
			"code":                     200,
			"_postman_previewlanguage": "json",
			"body":                     sample,
		}}
	}
	return item
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

// Postman Collection v2.x 结构（只取需要的字段）。
type postmanCollection struct {
	Info struct {
		Name string `json:"name"`
	} `json:"info"`
	Item []postmanItem `json:"item"`
}

type postmanItem struct {
	Name    string        `json:"name"`
	Item    []postmanItem `json:"item"`
	Request *postmanReq   `json:"request"`
}

type postmanReq struct {
	Method      string          `json:"method"`
	URL         json.RawMessage `json:"url"`
	Header      []postmanKV     `json:"header"`
	Body        *postmanBody    `json:"body"`
	Description json.RawMessage `json:"description"`
}

type postmanKV struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled"`
}

type postmanBody struct {
	Mode string `json:"mode"`
	Raw  string `json:"raw"`
}

// ImportPostman 导入 Postman 集合（JSON 文本）。
func (s *Service) ImportPostman(ctx context.Context, userID uint64, typ int8, projectID uint64, strategy string, body []byte) (ImportResult, error) {
	if err := s.ensureProjectPermission(ctx, userID, typ, projectID, model.PermCreate); err != nil {
		return ImportResult{}, err
	}
	var col postmanCollection
	if err := json.Unmarshal(body, &col); err != nil {
		return ImportResult{}, newBiz(errcode.CodeParamError, "Postman 集合解析失败：%v", err)
	}
	if len(col.Item) == 0 {
		return ImportResult{}, newBiz(errcode.CodeParamError, "集合里没有请求（item 为空），请确认导出的是 Collection v2.x")
	}
	keys, err := s.prepareImport(ctx, projectID, strategy)
	if err != nil {
		return ImportResult{}, err
	}
	groups, err := s.repo.ListGroups(ctx, projectID, model.GroupTypeAPI)
	if err != nil {
		return ImportResult{}, err
	}
	ensure := s.postmanGroupEnsurer(ctx, projectID, &groups)

	var res ImportResult
	var walkErr error
	var walk func(items []postmanItem, parent uint64) error
	walk = func(items []postmanItem, parent uint64) error {
		for _, it := range items {
			if len(it.Item) > 0 {
				name := strings.TrimSpace(it.Name)
				if name == "" {
					name = "未命名目录"
				}
				gid, err := ensure(parent, name)
				if err != nil {
					return err
				}
				if err := walk(it.Item, gid); err != nil {
					return err
				}
				continue
			}
			if it.Request == nil {
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
			if err := s.repo.CreateApi(ctx, a); err != nil {
				return err
			}
			keys[key] = struct{}{}
			res.Imported++
		}
		return nil
	}
	if err := walk(col.Item, 0); err != nil {
		walkErr = err
	}
	if walkErr != nil {
		return res, walkErr
	}
	s.recordOp(ctx, projectID, userID, projectID, model.LogObjectAPI, "import",
		"导入 Postman 接口 "+itoa(res.Imported)+" 条（跳过 "+itoa(res.Skipped)+"，失败 "+itoa(res.Failed)+"）")
	return res, nil
}

// postmanGroupEnsurer 惰性建分组：同名同父直接复用（与 Bruno 导入一致）。
func (s *Service) postmanGroupEnsurer(ctx context.Context, projectID uint64, groups *[]model.Group) func(parent uint64, name string) (uint64, error) {
	return func(parent uint64, name string) (uint64, error) {
		for _, g := range *groups {
			if g.PID == parent && g.Title == name {
				return g.ID, nil
			}
		}
		g := &model.Group{ProjectID: projectID, PID: parent, Title: name, Type: model.GroupTypeAPI}
		if err := s.repo.CreateGroup(ctx, g); err != nil {
			return 0, err
		}
		*groups = append(*groups, *g)
		return g.ID, nil
	}
}

// buildPostmanApi 把 Postman 请求转成接口记录；method/url 缺失时返回 (nil, nil)。
func buildPostmanApi(projectID, groupID uint64, item postmanItem) (*model.Api, error) {
	req := item.Request
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	raw := postmanURL(req.URL)
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
	for _, kv := range queryPairs(raw) {
		params = append(params, apiParamRow{FieldName: kv[0], Type: "string", Example: kv[1]})
	}
	if req.Body != nil && strings.EqualFold(strings.TrimSpace(req.Body.Mode), "raw") && strings.TrimSpace(req.Body.Raw) != "" {
		var v any
		if err := json.Unmarshal([]byte(req.Body.Raw), &v); err != nil {
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
	for _, h := range req.Header {
		if h.Disabled || strings.TrimSpace(h.Key) == "" {
			continue
		}
		headers = append(headers, apiHeaderRow{Name: h.Key, Value: h.Value})
	}
	if len(headers) > 0 {
		if b, err := json.Marshal(headers); err == nil {
			a.HttpRequestHeader = string(b)
		}
	}
	return a, nil
}

// postmanURL 取请求 URL 文本，兼容字符串与 `{ raw }` 两种写法。
func postmanURL(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var obj struct {
		Raw string `json:"raw"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return strings.TrimSpace(obj.Raw)
	}
	return ""
}

// queryPairs 从 URL 查询串提取键值对（按出现顺序，值做百分号解码前的原样保留）。
func queryPairs(raw string) [][2]string {
	i := strings.Index(raw, "?")
	if i < 0 {
		return nil
	}
	query := raw[i+1:]
	if j := strings.IndexAny(query, "#"); j >= 0 {
		query = query[:j]
	}
	out := make([][2]string, 0, 4)
	for _, pair := range strings.Split(query, "&") {
		if pair == "" {
			continue
		}
		key, value := pair, ""
		if k := strings.Index(pair, "="); k >= 0 {
			key, value = pair[:k], pair[k+1:]
		}
		if key == "" {
			continue
		}
		out = append(out, [2]string{key, value})
	}
	return out
}
