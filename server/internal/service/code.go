// 代码片段：依据接口记录生成 cURL / fetch / axios / Go / Python 请求示例。
// 变量处理与 cURL 一致：resolve=false 保留 {{占位符}}，resolve=true 用当前环境实际值
// （含敏感值，仅服务端渲染，不经接口回传单个变量明文）；涉及敏感值的导出写操作日志。
package service

import (
	"context"
	"strings"

	"github.com/zqstudio/api-doc-go-share/codegen"

	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
)

// 支持的代码语言（值取自共享包 codegen，两端一致）。
const (
	CodeLangCurl   = string(codegen.LangCurl)
	CodeLangFetch  = string(codegen.LangFetch)
	CodeLangAxios  = string(codegen.LangAxios)
	CodeLangGo     = string(codegen.LangGo)
	CodeLangPython = string(codegen.LangPython)
)

// CodeSnippet 生成接口的代码片段；resolve 为真时用当前环境变量值替换占位符。
// 权限、审计与 model.Api → codegen.Request 的映射留在服务端，纯渲染由共享包 codegen 承担。
func (s *Service) CodeSnippet(ctx context.Context, userID uint64, apiID, envID uint64, lang string, resolve bool) (string, error) {
	a, err := s.repo.GetApi(ctx, apiID)
	if err != nil {
		return "", err
	}
	lang = strings.ToLower(strings.TrimSpace(lang))
	if lang == "" {
		lang = CodeLangCurl
	}
	parsed, ok := codegen.ParseLang(lang)
	if !ok {
		return "", newBiz(errcode.CodeParamError, "不支持的代码语言：%s", lang)
	}

	// resolve=false 时用空表渲染 → 占位符原样保留。
	table := map[string]model.Variable{}
	if resolve {
		table, err = s.resolveTable(ctx, a.ProjectID, a.ID, envID)
		if err != nil {
			return "", err
		}
	}
	rawURL := a.URL
	rawBody := paramsBody(a.HttpRequestParams)

	// 审计：扫描 URL / 请求头 / 请求体中引用到的敏感变量（空引用不记日志）。
	if resolve {
		rawTexts := []string{rawURL, rawBody}
		for _, h := range parseRows(a.HttpRequestHeader) {
			if value, ok := h["value"].(string); ok {
				rawTexts = append(rawTexts, value)
			}
		}
		s.auditVarExport(ctx, userID, a, referencedSecretVars(rawTexts, table))
	}

	return codegen.Snippet(parsed, codegen.Request{
		Method:  strings.ToUpper(strings.TrimSpace(a.HttpMethodType)),
		URL:     renderVars(rawURL, table),
		Headers: codeHeaders(a, table),
		Body:    renderVars(rawBody, table),
	})
}

// auditVarExport 记录"含敏感变量的导出"（只记接口名与变量名，不记值）。
func (s *Service) auditVarExport(ctx context.Context, userID uint64, a *model.Api, secretNames []string) {
	if len(secretNames) == 0 {
		return
	}
	s.recordOp(ctx, a.ProjectID, userID, a.ID, model.LogObjectVariable, "export",
		"导出含敏感变量的请求代码：接口 "+a.ApiName+"；变量 "+strings.Join(secretNames, ", "))
}

// codeHeaders 解析并渲染请求头（跳过空名；名称不做 trim，保证 cURL 输出与历史实现逐字节一致）。
func codeHeaders(a *model.Api, table map[string]model.Variable) []codegen.KV {
	out := make([]codegen.KV, 0)
	for _, h := range parseRows(a.HttpRequestHeader) {
		name, _ := h["name"].(string)
		value, _ := h["value"].(string)
		if name == "" {
			continue
		}
		out = append(out, codegen.KV{Name: name, Value: renderVars(value, table)})
	}
	return out
}
