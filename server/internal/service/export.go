package service

import (
	"context"
	"encoding/json"
	"strings"

	shopenapi "github.com/zqstudio/api-doc-go-share/openapi"

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

	doc := &shopenapi.Document{Title: docTitle, Version: docVersion}
	for i := range apis {
		doc.Ops = append(doc.Ops, openAPIOperation(&apis[i], titles[apis[i].GroupID]))
	}
	return doc.Encode(), nil
}

// openAPIOperation 接口记录 → 中立操作（分组名进 tags；方法空则按 GET 导出）。
func openAPIOperation(a *model.Api, groupTitle string) shopenapi.Operation {
	op := shopenapi.Operation{
		Path:        a.URL,
		Method:      strings.TrimSpace(a.HttpMethodType),
		Summary:     a.ApiName,
		Description: a.Description,
		Params:      openAPIParams(a.HttpRequestParams),
		Responses:   openAPIResponses(a),
	}
	if op.Method == "" {
		op.Method = "get"
	}
	if groupTitle != "" {
		op.Tags = []string{groupTitle}
	}
	return op
}

// openAPIParams 存储的 JSON 参数数组 → OpenAPI parameters；解析失败返回 nil。
func openAPIParams(raw string) []shopenapi.Param {
	if raw == "" {
		return nil
	}
	var items []map[string]any
	if err := json.Unmarshal([]byte(raw), &items); err != nil || len(items) == 0 {
		return nil
	}
	out := make([]shopenapi.Param, 0, len(items))
	for _, it := range items {
		name, _ := it["fieldName"].(string)
		if name == "" {
			continue
		}
		p := shopenapi.Param{Name: name, In: "query", Required: it["required"] == true}
		if d, ok := it["description"].(string); ok {
			p.Description = d
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// openAPIResponses 响应样例 → 中立响应（样例原文交给内核判断能否作为 JSON 示例）。
func openAPIResponses(a *model.Api) []shopenapi.Response {
	return []shopenapi.Response{{
		Code:        "200",
		Description: a.HttpReturnType,
		ExampleJSON: a.HttpReturnSample,
	}}
}

// ExportCurl 依据接口记录生成可执行的 cURL 命令。
// envID/resolve 为真时用当前环境变量值替换 {{占位符}}（resolve=false 保持历史行为：占位符原样保留）。
func (s *Service) ExportCurl(ctx context.Context, userID uint64, apiID, envID uint64, resolve bool) (string, error) {
	return s.CodeSnippet(ctx, userID, apiID, envID, CodeLangCurl, resolve)
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
