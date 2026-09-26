// Mock 服务：按「请求方法 + 路径」匹配项目内接口，返回其响应样例作为响应体（免登录，供前端联调）。
// 样例中的 {{变量}} 用项目通用变量（env_id=0）渲染，方便同一份样例覆盖多环境。
package service

import (
	"context"
	"encoding/json"
	"strings"

	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
)

// MockResult mock 响应（状态码 / Content-Type / 响应体）。
type MockResult struct {
	Status      int
	ContentType string
	Body        string
}

// MockResponse 匹配接口并返回 mock 数据；方法与路径都未命中时返回 404 JSON。
func (s *Service) MockResponse(ctx context.Context, projectID uint64, method, reqPath string) (*MockResult, error) {
	if projectID == 0 {
		return nil, newBiz(errcode.CodeParamError, "缺少项目 ID")
	}
	if _, err := s.repo.ProjectByID(ctx, projectID); err != nil {
		return nil, newBiz(errcode.CodeFailed, "项目不存在或已删除")
	}
	apis, err := s.repo.ListApisByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	want := normalizeMockPath(reqPath)
	var fallback *model.Api
	for i := range apis {
		if normalizeMockPath(apiURLPath(apis[i].URL)) != want {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(apis[i].HttpMethodType), strings.TrimSpace(method)) {
			return s.mockBody(ctx, &apis[i])
		}
		if fallback == nil {
			fallback = &apis[i]
		}
	}
	if fallback != nil {
		return s.mockBody(ctx, fallback)
	}
	body := `{"code":404,"msg":"未匹配到接口，请在文档中确认请求方法与路径","path":"` + want + `"}`
	return &MockResult{Status: 404, ContentType: "application/json; charset=utf-8", Body: body}, nil
}

// mockBody 以响应样例为响应体：JSON 合法则按 JSON 返回，否则按纯文本返回。
func (s *Service) mockBody(ctx context.Context, a *model.Api) (*MockResult, error) {
	sample := strings.TrimSpace(a.HttpReturnSample)
	if sample == "" {
		return &MockResult{
			Status:      200,
			ContentType: "application/json; charset=utf-8",
			Body:        `{"code":200,"msg":"该接口未配置响应样例","data":null}`,
		}, nil
	}
	if table, err := s.resolveTable(ctx, a.ProjectID, a.ID, 0); err == nil && len(table) > 0 {
		sample = renderVars(sample, table)
	}
	if json.Valid([]byte(sample)) {
		return &MockResult{Status: 200, ContentType: "application/json; charset=utf-8", Body: sample}, nil
	}
	return &MockResult{Status: 200, ContentType: "text/plain; charset=utf-8", Body: sample}, nil
}

// apiURLPath 取接口 URL 的路径部分（去掉 scheme / host / query / fragment）。
func apiURLPath(raw string) string {
	u := strings.TrimSpace(raw)
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+3:]
		if j := strings.Index(u, "/"); j >= 0 {
			u = u[j:]
		} else {
			u = "/"
		}
	}
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	return u
}

// normalizeMockPath 归一化路径：剔除空段，统一为以 "/" 开头且无尾斜杠。
func normalizeMockPath(p string) string {
	p = strings.TrimSpace(p)
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	parts := make([]string, 0, 8)
	for _, part := range strings.Split(p, "/") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return "/" + strings.Join(parts, "/")
}
