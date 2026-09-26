// 代码片段：依据接口记录生成 cURL / fetch / axios / Go / Python 请求示例。
// 变量处理与 cURL 一致：resolve=false 保留 {{占位符}}，resolve=true 用当前环境实际值
// （含敏感值，仅服务端渲染，不经接口回传单个变量明文）；涉及敏感值的导出写操作日志。
package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
)

// 支持的代码语言。
const (
	CodeLangCurl   = "curl"
	CodeLangFetch  = "fetch"
	CodeLangAxios  = "axios"
	CodeLangGo     = "go"
	CodeLangPython = "python"
)

// codeHeader 渲染后的请求头。
type codeHeader struct {
	Name  string
	Value string
}

// CodeSnippet 生成接口的代码片段；resolve 为真时用当前环境变量值替换占位符。
func (s *Service) CodeSnippet(ctx context.Context, userID uint64, apiID, envID uint64, lang string, resolve bool) (string, error) {
	a, err := s.repo.GetApi(ctx, apiID)
	if err != nil {
		return "", err
	}
	lang = strings.ToLower(strings.TrimSpace(lang))
	if lang == "" {
		lang = CodeLangCurl
	}
	switch lang {
	case CodeLangCurl, CodeLangFetch, CodeLangAxios, CodeLangGo, CodeLangPython:
	default:
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
	method := strings.ToUpper(strings.TrimSpace(a.HttpMethodType))
	if method == "" {
		method = "GET"
	}
	rawURL := a.URL
	rawBody := paramsBody(a.HttpRequestParams)
	url := renderVars(rawURL, table)
	body := renderVars(rawBody, table)
	headers := codeHeaders(a, table)

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

	switch lang {
	case CodeLangFetch:
		return fetchSnippet(method, url, headers, body), nil
	case CodeLangAxios:
		return axiosSnippet(method, url, headers, body), nil
	case CodeLangGo:
		return goSnippet(method, url, headers, body), nil
	case CodeLangPython:
		return pythonSnippet(method, url, headers, body), nil
	}
	return curlSnippet(method, url, headers, body), nil
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
func codeHeaders(a *model.Api, table map[string]model.Variable) []codeHeader {
	out := make([]codeHeader, 0)
	for _, h := range parseRows(a.HttpRequestHeader) {
		name, _ := h["name"].(string)
		value, _ := h["value"].(string)
		if name == "" {
			continue
		}
		out = append(out, codeHeader{Name: name, Value: renderVars(value, table)})
	}
	return out
}

// curlSnippet 生成可执行 cURL；输出格式与历史实现保持一致（回归点）。
func curlSnippet(method, url string, headers []codeHeader, body string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "curl -X %s '%s'", method, url)
	for _, h := range headers {
		fmt.Fprintf(&b, " \\\n  -H '%s: %s'", h.Name, h.Value)
	}
	if body != "" {
		fmt.Fprintf(&b, " \\\n  -d '%s'", body)
	}
	return b.String()
}

// fetchSnippet 生成浏览器/Node 的 fetch 示例。
func fetchSnippet(method, url string, headers []codeHeader, body string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "const res = await fetch('%s', {\n  method: '%s',", jsQuote(url), method)
	if len(headers) > 0 {
		b.WriteString("\n  headers: {")
		for _, h := range headers {
			fmt.Fprintf(&b, "\n    '%s': '%s',", jsQuote(h.Name), jsQuote(h.Value))
		}
		b.WriteString("\n  },")
	}
	if body != "" {
		fmt.Fprintf(&b, "\n  body: JSON.stringify(%s),", body)
	}
	b.WriteString("\n});\nconst data = await res.json();\nconsole.log(data);")
	return b.String()
}

// axiosSnippet 生成 axios（TS）示例。
func axiosSnippet(method, url string, headers []codeHeader, body string) string {
	var b strings.Builder
	b.WriteString("import axios from 'axios';\n\n")
	b.WriteString("const { data } = await axios.request({\n")
	fmt.Fprintf(&b, "  method: '%s',\n  url: '%s',", method, jsQuote(url))
	if len(headers) > 0 {
		b.WriteString("\n  headers: {")
		for _, h := range headers {
			fmt.Fprintf(&b, "\n    '%s': '%s',", jsQuote(h.Name), jsQuote(h.Value))
		}
		b.WriteString("\n  },")
	}
	if body != "" {
		fmt.Fprintf(&b, "\n  data: %s,", body)
	}
	b.WriteString("\n});\nconsole.log(data);")
	return b.String()
}

// goSnippet 生成 Go net/http 示例。
func goSnippet(method, url string, headers []codeHeader, body string) string {
	reader := "nil"
	if body != "" {
		reader = "strings.NewReader(" + strconv.Quote(body) + ")"
	}
	var b strings.Builder
	b.WriteString("package main\n\nimport (\n\t\"fmt\"\n\t\"io\"\n\t\"net/http\"\n")
	if body != "" {
		b.WriteString("\t\"strings\"\n")
	}
	b.WriteString(")\n\nfunc main() {\n")
	fmt.Fprintf(&b, "\treq, err := http.NewRequest(%s, %s, %s)\n", strconv.Quote(method), strconv.Quote(url), reader)
	b.WriteString("\tif err != nil {\n\t\tpanic(err)\n\t}\n")
	for _, h := range headers {
		fmt.Fprintf(&b, "\treq.Header.Set(%s, %s)\n", strconv.Quote(h.Name), strconv.Quote(h.Value))
	}
	b.WriteString("\tres, err := http.DefaultClient.Do(req)\n\tif err != nil {\n\t\tpanic(err)\n\t}\n")
	b.WriteString("\tdefer res.Body.Close()\n\tdata, _ := io.ReadAll(res.Body)\n\tfmt.Println(res.Status, string(data))\n}")
	return b.String()
}

// pythonSnippet 生成 Python requests 示例。
func pythonSnippet(method, url string, headers []codeHeader, body string) string {
	var b strings.Builder
	b.WriteString("import requests\n\n")
	fmt.Fprintf(&b, "url = '%s'\n", jsQuote(url))
	if len(headers) > 0 {
		b.WriteString("headers = {\n")
		for _, h := range headers {
			fmt.Fprintf(&b, "    '%s': '%s',\n", jsQuote(h.Name), jsQuote(h.Value))
		}
		b.WriteString("}\n")
	} else {
		b.WriteString("headers = {}\n")
	}
	if body != "" {
		fmt.Fprintf(&b, "payload = %s\n", body)
	}
	// 无请求体时不带 data 参数。
	if body != "" {
		fmt.Fprintf(&b, "res = requests.request('%s', url, headers=headers, data=payload)\n", method)
	} else {
		fmt.Fprintf(&b, "res = requests.request('%s', url, headers=headers)\n", method)
	}
	b.WriteString("print(res.status_code, res.text)")
	return b.String()
}

// jsQuote 转义单引号字符串（JS/Python 通用）。
func jsQuote(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	return strings.ReplaceAll(s, "'", "\\'")
}
