package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
)

// TestCodeSnippetVariables 覆盖多语言代码片段与变量渲染：
// resolve=false 保留占位符（回归历史 cURL 行为），resolve=true 用当前环境实际值替换（未定义者原样保留），
// 涉及敏感值时写审计日志（不记值），非法语言返回参数错误。
func TestCodeSnippetVariables(t *testing.T) {
	f := newVarFixture(t)
	ctx := context.Background()
	headers := `[{"name":"Authorization","value":"Bearer {{token}}"},{"name":"X-Host","value":"{{host}}"},{"name":"X-Secret","value":"{{secret}}"}]`
	api := f.newAPI(t, "code-api", "https://{{host}}/api/user/list?k={{kw}}", headers)

	f.newVar(t, dto.VariableReq{Scope: model.VarScopeRequest, ProjectID: f.proj.ID, ApiID: api.ID, Name: "token", Value: strPtr("t-123")})
	f.newVar(t, dto.VariableReq{Scope: model.VarScopeProject, ProjectID: f.proj.ID, EnvID: f.dev.ID, Name: "host", Value: strPtr("dev.example.com")})
	f.newVar(t, dto.VariableReq{Scope: model.VarScopeProject, ProjectID: f.proj.ID, EnvID: f.dev.ID, Name: "secret", Value: strPtr("s3cret-val"), IsSecret: model.VarSecret})

	// 1) 保留占位符：与历史 cURL 行为一致。
	raw, err := f.s.ExportCurl(ctx, f.admin.ID, api.ID, f.dev.ID, false)
	if err != nil {
		t.Fatalf("ExportCurl(resolve=false): %v", err)
	}
	if !strings.Contains(raw, "https://{{host}}/api/user/list?k={{kw}}") || strings.Contains(raw, "dev.example.com") {
		t.Fatalf("resolve=false 应保留占位符，got:\n%s", raw)
	}

	// 2) 替换为实际值：已知变量被替换，未定义（kw）原样保留。
	got, err := f.s.ExportCurl(ctx, f.admin.ID, api.ID, f.dev.ID, true)
	if err != nil {
		t.Fatalf("ExportCurl(resolve=true): %v", err)
	}
	for _, want := range []string{"https://dev.example.com/api/user/list?k={{kw}}", "Bearer t-123", "X-Host: dev.example.com"} {
		if !strings.Contains(got, want) {
			t.Fatalf("resolve=true 应包含 %q，got:\n%s", want, got)
		}
	}
	// 敏感值在服务端渲染时使用原文（客户端只拿到成品命令）。
	if !strings.Contains(got, "s3cret-val") {
		t.Fatalf("resolve=true 应使用敏感值原文渲染，got:\n%s", got)
	}

	// 3) 其它语言均可生成，且变量同样被替换。
	for _, lang := range []string{CodeLangFetch, CodeLangAxios, CodeLangGo, CodeLangPython} {
		code, err := f.s.CodeSnippet(ctx, f.admin.ID, api.ID, f.dev.ID, lang, true)
		if err != nil {
			t.Fatalf("CodeSnippet(%s): %v", lang, err)
		}
		if !strings.Contains(code, "dev.example.com") || !strings.Contains(code, "Bearer t-123") {
			t.Fatalf("%s 片段应包含已渲染的 URL 与请求头，got:\n%s", lang, code)
		}
	}

	// 4) 审计：涉及敏感变量的导出落操作日志，且只记变量名不记值。
	var logged int64
	if err := f.s.repo.DB().Raw(
		"SELECT COUNT(*) FROM operation_log WHERE action = 'export' AND content LIKE '%secret%' AND content NOT LIKE '%s3cret-val%'",
	).Scan(&logged).Error; err != nil {
		t.Fatalf("查询操作日志: %v", err)
	}
	if logged == 0 {
		t.Fatal("含敏感变量的导出应写审计日志（只记变量名）")
	}

	// 5) 非法语言返回参数错误。
	_, err = f.s.CodeSnippet(ctx, f.admin.ID, api.ID, f.dev.ID, "ruby", false)
	var be *BizError
	if !errors.As(err, &be) || be.Code != errcode.CodeParamError {
		t.Fatalf("非法语言应返回参数错误，got %v", err)
	}
}
