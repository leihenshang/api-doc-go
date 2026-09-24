package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"apidoc/internal/model"
	"apidoc/internal/pkg/page"
)

// OpenAPIExport 将项目（可选限定分组）下的接口生成标准 OpenAPI 3.0 文档（可 JSON 序列化）。
// 与导入（ImportOpenAPI）互逆：summary=接口名、tags=分组名、parameters=请求参数、
// responses.200.content 取响应样例。
func (s *Service) OpenAPIExport(ctx context.Context, projectID, groupID uint64) (map[string]any, error) {
	q := page.Query{PageNum: 1, PageSize: 200}
	q.Normalize()
	apis, _, err := s.repo.ListApis(ctx, projectID, groupID, 0, "", q)
	if err != nil {
		return nil, err
	}
	titles := map[uint64]string{}
	if groups, err := s.repo.ListGroups(ctx, projectID, -1); err == nil {
		for _, g := range groups {
			titles[g.ID] = g.Title
		}
	}
	docTitle, docVersion := "apidoc", "1.0"
	if p, err := s.repo.ProjectByID(ctx, projectID); err == nil {
		docTitle = p.Title
		if p.Version != "" {
			docVersion = p.Version
		}
	}

	paths := map[string]any{}
	for i := range apis {
		a := &apis[i]
		method := strings.ToLower(strings.TrimSpace(a.HttpMethodType))
		if method == "" {
			method = "get"
		}
		op := map[string]any{
			"summary":   a.ApiName,
			"responses": exportResponses(a),
		}
		if t := titles[a.GroupID]; t != "" {
			op["tags"] = []string{t}
		}
		if a.Description != "" {
			op["description"] = a.Description
		}
		if params := exportParameters(a.HttpRequestParams); params != nil {
			op["parameters"] = params
		}
		pathItem, ok := paths[a.URL].(map[string]any)
		if !ok {
			pathItem = map[string]any{}
			paths[a.URL] = pathItem
		}
		pathItem[method] = op
	}

	return map[string]any{
		"openapi": "3.0.0",
		"info":    map[string]any{"title": docTitle, "version": docVersion},
		"paths":   paths,
	}, nil
}

// exportParameters 将存储的 JSON 参数数组转为 OpenAPI parameters；解析失败返回 nil。
func exportParameters(raw string) any {
	if raw == "" {
		return nil
	}
	var items []map[string]any
	if err := json.Unmarshal([]byte(raw), &items); err != nil || len(items) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		name, _ := it["fieldName"].(string)
		if name == "" {
			continue
		}
		p := map[string]any{"name": name, "in": "query", "required": it["required"] == true}
		if d, ok := it["description"].(string); ok && d != "" {
			p["description"] = d
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// exportResponses 构造 responses.200；样例列可解析为 JSON 时嵌入 content.example。
func exportResponses(a *model.Api) map[string]any {
	desc := a.HttpReturnType
	if desc == "" {
		desc = "成功"
	}
	resp := map[string]any{"description": desc}
	if a.HttpReturnSample != "" {
		var sample any
		if err := json.Unmarshal([]byte(a.HttpReturnSample), &sample); err == nil {
			resp["content"] = map[string]any{
				"application/json": map[string]any{"example": sample},
			}
		}
	}
	return map[string]any{"200": resp}
}

// ExportCurl 依据接口记录生成可执行的 cURL 命令。
func (s *Service) ExportCurl(ctx context.Context, apiID uint64) (string, error) {
	a, err := s.repo.GetApi(ctx, apiID)
	if err != nil {
		return "", err
	}
	method := strings.ToUpper(strings.TrimSpace(a.HttpMethodType))
	if method == "" {
		method = "GET"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "curl -X %s '%s'", method, a.URL)
	if items := parseRows(a.HttpRequestHeader); items != nil {
		for _, h := range items {
			name, _ := h["name"].(string)
			value, _ := h["value"].(string)
			if name != "" {
				fmt.Fprintf(&b, " \\\n  -H '%s: %s'", name, value)
			}
		}
	}
	if body := paramsBody(a.HttpRequestParams); body != "" {
		fmt.Fprintf(&b, " \\\n  -d '%s'", body)
	}
	return b.String(), nil
}

// parseRows 解析对象数组列；失败返回 nil。
func parseRows(raw string) []map[string]any {
	if raw == "" {
		return nil
	}
	var out []map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

// paramsBody 以参数行的 example（缺省用类型占位）构造请求体 JSON。
func paramsBody(raw string) string {
	items := parseRows(raw)
	if len(items) == 0 {
		return ""
	}
	body := map[string]any{}
	for _, it := range items {
		name, _ := it["fieldName"].(string)
		if name == "" {
			continue
		}
		if ex, ok := it["example"].(string); ok && ex != "" {
			body[name] = ex
			continue
		}
		t, _ := it["type"].(string)
		body[name] = "<" + t + ">"
	}
	if len(body) == 0 {
		return ""
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return ""
	}
	return string(buf)
}
