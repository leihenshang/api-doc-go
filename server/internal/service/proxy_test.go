package service

// RM-01 在线调试回归：
//   - 变量渲染（URL/头/正文/表单）与认证；
//   - 无明文权限的调用方在响应里看不到敏感值（目标服务回显也会被抹掉）；
//   - 审计只记方法/主机/路径与变量名；
//   - SSRF 拦截回环/内网地址，非 http/https 直接拒绝。

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
	"apidoc/internal/pkg/page"
)

// newProxyEchoServer 回显方法、查询串、认证、请求体与 Content-Type。
func newProxyEchoServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Echo-Auth", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(map[string]string{
			"method":      r.Method,
			"path":        r.URL.Path,
			"query":       r.URL.RawQuery,
			"auth":        r.Header.Get("Authorization"),
			"body":        string(body),
			"contentType": r.Header.Get("Content-Type"),
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// withProxyClient 让代理直连本地 httptest（仅测试用；生产仍走 SSRF 校验）。
func withProxyClient(t *testing.T, srv *httptest.Server) {
	t.Helper()
	old := proxyClientProvider
	proxyClientProvider = func() *http.Client {
		return &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
				},
			},
		}
	}
	t.Cleanup(func() { proxyClientProvider = old })
}

// TestProxySendRendersAndGatesSecrets 变量渲染、认证、敏感值权限门禁与审计。
func TestProxySendRendersAndGatesSecrets(t *testing.T) {
	srv := newProxyEchoServer(t)
	withProxyClient(t, srv)
	f := newVarFixture(t)
	f.newVar(t, dto.VariableReq{
		Scope: model.VarScopeProject, ProjectID: f.proj.ID, EnvID: f.dev.ID,
		Name: "token", Value: strPtr("s3cret-token"), IsSecret: model.VarSecret,
	})
	bob := f.registerReadonly(t, "bob-proxy")
	req := dto.ProxySendReq{
		ProjectID: f.proj.ID, EnvID: f.dev.ID, Method: "POST",
		URL:      "http://example.test/echo?q={{token}}",
		Headers:  []dto.ProxyKV{{Name: "Authorization", Value: "Bearer {{token}}"}},
		BodyType: "json", Body: `{"token":"{{token}}"}`,
	}

	adminResp, err := f.s.ProxySend(f.ctx, f.admin.ID, f.admin.Type, req)
	if err != nil {
		t.Fatalf("ProxySend(admin): %v", err)
	}
	if adminResp.Status != 200 || !strings.Contains(adminResp.Body, "s3cret-token") {
		t.Fatalf("管理员应看到目标服务回显的明文: status=%d body=%s", adminResp.Status, adminResp.Body)
	}
	var echo map[string]string
	if err := json.Unmarshal([]byte(adminResp.Body), &echo); err != nil {
		t.Fatalf("echo 解析失败: %v", err)
	}
	if echo["method"] != "POST" || !strings.Contains(echo["query"], "q=s3cret-token") ||
		echo["auth"] != "Bearer s3cret-token" || !strings.Contains(echo["body"], "s3cret-token") ||
		echo["contentType"] != "application/json" {
		t.Fatalf("变量渲染/认证/正文不符合预期: %+v", echo)
	}

	// 无明文权限的成员仍可调试，但敏感占位符不替换（避免把 secret 发往任意目标）。
	bobResp, err := f.s.ProxySend(f.ctx, bob.ID, bob.Type, req)
	if err != nil {
		t.Fatalf("ProxySend(readonly): %v", err)
	}
	if strings.Contains(bobResp.Body, "s3cret-token") {
		t.Fatalf("只读用户不应在响应体看到明文: %s", bobResp.Body)
	}
	for _, h := range bobResp.Headers {
		if strings.Contains(h.Value, "s3cret-token") {
			t.Fatalf("只读用户不应在响应头看到明文: %s=%s", h.Name, h.Value)
		}
	}
	if !strings.Contains(bobResp.Body, "{{token}}") {
		t.Fatalf("无明文权限时敏感占位符应保持原样: %s", bobResp.Body)
	}

	// 审计：只记方法与主机/路径，并带上敏感变量名。
	logs, _, err := f.s.repo.ListLogs(f.ctx, f.proj.ID, 0, model.LogObjectAPI, page.Query{PageNum: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListLogs: %v", err)
	}
	found := false
	for _, lg := range logs {
		if lg.Action == "debug" && strings.Contains(lg.Content, "在线调试") && strings.Contains(lg.Content, "token") {
			found = true
			if strings.Contains(lg.Content, "s3cret-token") {
				t.Fatalf("审计不应记录敏感值: %s", lg.Content)
			}
		}
	}
	if !found {
		t.Fatalf("未找到在线调试审计: %+v", logs)
	}
}

// TestProxySendRejectsSSRFAndBadScheme 回环地址与非 http/https 必须被拒绝。
func TestProxySendRejectsSSRFAndBadScheme(t *testing.T) {
	f := newVarFixture(t)
	_, err := f.s.ProxySend(f.ctx, f.admin.ID, f.admin.Type, dto.ProxySendReq{
		ProjectID: f.proj.ID, Method: "GET", URL: "http://127.0.0.1:1/",
	})
	var be *BizError
	if !errors.As(err, &be) || be.Code != errcode.CodeForbidden || !strings.Contains(be.Msg, "不允许访问") {
		t.Fatalf("回环地址应被 SSRF 拦截，got %v", err)
	}

	_, err = f.s.ProxySend(f.ctx, f.admin.ID, f.admin.Type, dto.ProxySendReq{
		ProjectID: f.proj.ID, Method: "GET", URL: "file:///etc/passwd",
	})
	if !errors.As(err, &be) || be.Code != errcode.CodeParamError {
		t.Fatalf("file:// 应被参数错误拒绝，got %v", err)
	}
}

// TestProxyClientRejectsCrossHostRedirect 跨主机跳转可能把认证/自定义密钥带给第三方。
func TestProxyClientRejectsCrossHostRedirect(t *testing.T) {
	c := newSafeProxyClient()
	start, _ := http.NewRequest(http.MethodGet, "http://example.test/a", nil)
	other, _ := http.NewRequest(http.MethodGet, "http://evil.test/b", nil)
	if err := c.CheckRedirect(other, []*http.Request{start}); err == nil {
		t.Fatal("跨主机跳转应被拒绝")
	}
	same, _ := http.NewRequest(http.MethodGet, "https://example.test/a", nil)
	if err := c.CheckRedirect(same, []*http.Request{start}); err != nil {
		t.Fatalf("同主机 http→https 应允许: %v", err)
	}
}

// TestIsBlockedProxyIP 覆盖常见内网/保留网段与公网放行。
func TestIsBlockedProxyIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "127.1.2.3", "10.0.0.1", "172.16.0.1", "192.168.1.1",
		"169.254.1.1", "100.64.0.1", "0.0.0.0", "192.0.2.1", "198.18.0.1",
		"::1", "fc00::1", "fe80::1",
	}
	for _, s := range blocked {
		if ip := net.ParseIP(s); ip == nil || !isBlockedProxyIP(ip) {
			t.Errorf("%s 应被拦截", s)
		}
	}
	for _, s := range []string{"8.8.8.8", "1.1.1.1", "2001:4860:4860::8888"} {
		if ip := net.ParseIP(s); ip == nil || isBlockedProxyIP(ip) {
			t.Errorf("%s 应允许", s)
		}
	}
}
