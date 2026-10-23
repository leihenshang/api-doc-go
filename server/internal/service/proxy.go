package service

// 在线调试（Web Try it）的请求代发（RM-01）。
//
// 设计要点：
//   - 只允许项目成员（PermQuery）使用；变量渲染复用 resolveTable/renderVars，
//     服务端加密的 secret（enc:v1:）不解析，读取权限不足时不回显明文；
//   - 代理必须做 SSRF 防护：仅 http/https、禁回环/内网/保留地址、每次连接校验 DNS 结果；
//   - 响应有超时与大小上限；请求/响应都不回显渲染后的请求内容，审计只记变量名不记值。

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
)

const (
	proxyTimeout          = 15 * time.Second
	proxyDialTimeout      = 5 * time.Second
	proxyMaxResponseBytes = 5 << 20 // 5 MB
	proxyMaxRedirects     = 5
	proxyUserAgent        = "api-doc-go-tryit/1.0"
)

// errProxyBlocked 标记 SSRF 拦截，便于 handler 映射成清晰文案。
var errProxyBlocked = errors.New("proxy: target address blocked")

// proxyMethods 允许代发的 HTTP 方法（不支持 CONNECT/TRACE 等）。
var proxyMethods = map[string]bool{
	http.MethodGet: true, http.MethodPost: true, http.MethodPut: true,
	http.MethodPatch: true, http.MethodDelete: true, http.MethodHead: true,
}

// blockedProxyCIDRs 额外禁止的保留网段（IsPrivate/IsLoopback 之外的 SSRF 高危段）。
var blockedProxyCIDRs = mustProxyCIDRs(
	"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24",
	"198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4",
	"2001:db8::/32", "100::/64",
)

// proxyClientProvider 构造代理 HTTP 客户端；测试可替换为直连 httptest 的实现。
var proxyClientProvider = newSafeProxyClient

// ProxySend 执行一次在线调试请求。
func (s *Service) ProxySend(ctx context.Context, userID uint64, typ int8, req dto.ProxySendReq) (*dto.ProxySendResp, error) {
	if err := s.ensureProjectPermission(ctx, userID, typ, req.ProjectID, model.PermQuery); err != nil {
		return nil, err
	}
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if !proxyMethods[method] {
		return nil, newBiz(errcode.CodeParamError, "不支持的请求方法：%s", req.Method)
	}
	// api_id 只用于取请求级变量：必须属于当前项目，避免拿别的项目的接口 id 套出变量。
	if req.APIID > 0 {
		a, err := s.repo.GetApi(ctx, req.APIID)
		if err != nil || a.ProjectID != req.ProjectID {
			return nil, newBiz(errcode.CodeParamError, "接口不属于该项目")
		}
	}
	// 明文 secret 只对有明文权限的调用方参与渲染：在线调试允许任意公网目标，
	// 若让无权限成员把 secret 发往自己控制的服务器，任何响应脱敏都可能被编码绕过。
	// 无权限成员仍可使用调试功能，但敏感占位符保持原样（与 code resolve=1 同口径）。
	reveal := s.canRevealValues(ctx, userID, typ, req.ProjectID)
	table, err := s.resolveTable(ctx, req.ProjectID, req.APIID, req.EnvID, reveal)
	if err != nil {
		return nil, err
	}
	target, err := buildProxyURL(req, table)
	if err != nil {
		return nil, err
	}
	headers := buildProxyHeaders(req.Headers, table)
	body, contentType, err := buildProxyBody(req, table)
	if err != nil {
		return nil, err
	}
	if contentType != "" && headers.Get("Content-Type") == "" {
		headers.Set("Content-Type", contentType)
	}
	if err := applyProxyAuth(req.Auth, headers, target, table); err != nil {
		return nil, err
	}
	if headers.Get("User-Agent") == "" {
		headers.Set("User-Agent", proxyUserAgent)
	}

	s.auditProxy(ctx, userID, req, table, target)
	resp, err := doProxyRequest(ctx, method, target, headers, body)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// buildProxyURL 渲染 URL 并合并查询参数；只允许 http/https 且禁止 URL 内嵌账号密码。
func buildProxyURL(req dto.ProxySendReq, table map[string]model.Variable) (*url.URL, error) {
	raw := strings.TrimSpace(renderVars(req.URL, table))
	if raw == "" {
		return nil, newBiz(errcode.CodeParamError, "URL 不能为空")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, newBiz(errcode.CodeParamError, "URL 格式不正确：%v", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, newBiz(errcode.CodeParamError, "仅支持 http/https，当前为 %q", u.Scheme)
	}
	if u.Host == "" {
		return nil, newBiz(errcode.CodeParamError, "URL 缺少主机名")
	}
	if u.User != nil {
		return nil, newBiz(errcode.CodeParamError, "URL 不能包含用户名/密码，请使用认证配置")
	}
	q := u.Query()
	for _, kv := range req.Query {
		if !proxyRowEnabled(kv.Enabled) {
			continue
		}
		name := strings.TrimSpace(renderVars(kv.Name, table))
		if name == "" {
			continue
		}
		q.Add(name, renderVars(kv.Value, table))
	}
	u.RawQuery = q.Encode()
	return u, nil
}

// buildProxyHeaders 渲染请求头，跳过 Host/Content-Length 等由 net/http 管理的头。
func buildProxyHeaders(rows []dto.ProxyKV, table map[string]model.Variable) http.Header {
	h := http.Header{}
	for _, kv := range rows {
		if !proxyRowEnabled(kv.Enabled) {
			continue
		}
		name := strings.TrimSpace(renderVars(kv.Name, table))
		if name == "" || isHopByHopHeader(name) {
			continue
		}
		h.Add(name, renderVars(kv.Value, table))
	}
	return h
}

// buildProxyBody 构造请求体；multipart 暂不支持（避免把本服务当文件上传通道）。
func buildProxyBody(req dto.ProxySendReq, table map[string]model.Variable) (io.Reader, string, error) {
	switch strings.ToLower(strings.TrimSpace(req.BodyType)) {
	case "", "none":
		return nil, "", nil
	case "json":
		return strings.NewReader(renderVars(req.Body, table)), "application/json", nil
	case "text", "raw":
		return strings.NewReader(renderVars(req.Body, table)), "text/plain; charset=utf-8", nil
	case "form":
		vals := url.Values{}
		for _, kv := range req.Form {
			if !proxyRowEnabled(kv.Enabled) {
				continue
			}
			name := strings.TrimSpace(renderVars(kv.Name, table))
			if name == "" {
				continue
			}
			vals.Add(name, renderVars(kv.Value, table))
		}
		return strings.NewReader(vals.Encode()), "application/x-www-form-urlencoded", nil
	case "multipart":
		return nil, "", newBiz(errcode.CodeParamError, "multipart 暂不支持在线调试，请改用表单或原始文本")
	default:
		return nil, "", newBiz(errcode.CodeParamError, "不支持的请求体类型：%s", req.BodyType)
	}
}

// applyProxyAuth 把认证配置写入请求头或查询参数（值同样支持 {{变量}}）。
func applyProxyAuth(auth *dto.ProxyAuth, h http.Header, u *url.URL, table map[string]model.Variable) error {
	if auth == nil {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(auth.Type)) {
	case "", "none", "inherit":
		return nil
	case "bearer":
		if token := renderVars(auth.Token, table); token != "" {
			h.Set("Authorization", "Bearer "+token)
		}
	case "basic":
		user, pass := renderVars(auth.Username, table), renderVars(auth.Password, table)
		if user == "" && pass == "" {
			return nil
		}
		h.Set("Authorization", "Basic "+basicAuthValue(user, pass))
	case "apikey":
		key := strings.TrimSpace(renderVars(auth.Key, table))
		if key == "" {
			return nil
		}
		value := renderVars(auth.Value, table)
		if strings.EqualFold(auth.In, "query") {
			q := u.Query()
			q.Set(key, value)
			u.RawQuery = q.Encode()
			return nil
		}
		h.Set(key, value)
	default:
		return newBiz(errcode.CodeParamError, "不支持的认证类型：%s", auth.Type)
	}
	return nil
}

// doProxyRequest 发出请求并读取受限大小的响应。
func doProxyRequest(ctx context.Context, method string, target *url.URL, headers http.Header, body io.Reader) (*dto.ProxySendResp, error) {
	reqCtx, cancel := context.WithTimeout(ctx, proxyTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, method, target.String(), body)
	if err != nil {
		return nil, newBiz(errcode.CodeParamError, "构造请求失败：%v", err)
	}
	req.Header = headers
	start := time.Now()
	resp, err := proxyClientProvider().Do(req)
	if err != nil {
		if errors.Is(err, errProxyBlocked) {
			return nil, newBiz(errcode.CodeForbidden, "目标地址不允许访问（内网/回环/保留地址）")
		}
		return nil, newBiz(errcode.CodeFailed, "请求失败：%v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, proxyMaxResponseBytes+1))
	if err != nil {
		return nil, newBiz(errcode.CodeFailed, "读取响应失败：%v", err)
	}
	truncated := len(raw) > proxyMaxResponseBytes
	if truncated {
		raw = raw[:proxyMaxResponseBytes]
	}
	return &dto.ProxySendResp{
		Status:      resp.StatusCode,
		StatusText:  resp.Status,
		Headers:     proxyResponseHeaders(resp.Header),
		Body:        string(raw),
		ContentType: resp.Header.Get("Content-Type"),
		DurationMS:  time.Since(start).Milliseconds(),
		Size:        int64(len(raw)),
		Truncated:   truncated,
		FinalURL:    resp.Request.URL.String(),
	}, nil
}

// newSafeProxyClient 构造带 SSRF 校验、超时与重定向上限的客户端。
func newSafeProxyClient() *http.Client {
	dialer := &net.Dialer{Timeout: proxyDialTimeout, KeepAlive: 30 * time.Second}
	tr := &http.Transport{
		// 不走环境代理：否则可能绕过 IP 校验。
		Proxy:                 nil,
		DisableKeepAlives:     true,
		TLSHandshakeTimeout:   proxyDialTimeout,
		ResponseHeaderTimeout: 10 * time.Second,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return safeProxyDial(ctx, dialer, network, addr)
		},
	}
	return &http.Client{
		Transport: tr,
		Timeout:   proxyTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= proxyMaxRedirects {
				return fmt.Errorf("重定向次数超过 %d 次", proxyMaxRedirects)
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("不支持的跳转协议：%s", req.URL.Scheme)
			}
			// 跨主机跳转可能把 Authorization/自定义 API Key 带给第三方，直接拒绝；
			// 同主机 http↔https 或路径跳转仍然允许。
			if len(via) > 0 && !strings.EqualFold(req.URL.Hostname(), via[0].URL.Hostname()) {
				return fmt.Errorf("跨主机重定向已拒绝：%s -> %s", via[0].URL.Host, req.URL.Host)
			}
			return nil
		},
	}
}

// safeProxyDial 先解析并校验全部 IP，再连接已校验的 IP（避免 DNS rebinding）。
func safeProxyDial(ctx context.Context, dialer *net.Dialer, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("解析域名 %s 失败：%w", host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("域名 %s 没有可用地址", host)
	}
	for _, ip := range ips {
		if isBlockedProxyIP(ip.IP) {
			return nil, fmt.Errorf("%w: %s", errProxyBlocked, ip.IP)
		}
	}
	var lastErr error
	for _, ip := range ips {
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// isBlockedProxyIP 判断地址是否属于回环/内网/链路本地/组播/保留段。
func isBlockedProxyIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsMulticast() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	for _, n := range blockedProxyCIDRs {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// proxyResponseHeaders 把响应头摊平成有序数组（保留重复项）。
func proxyResponseHeaders(h http.Header) []dto.ProxyHeader {
	out := make([]dto.ProxyHeader, 0, len(h))
	for name, values := range h {
		for _, v := range values {
			out = append(out, dto.ProxyHeader{Name: name, Value: v})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].Value < out[j].Value
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// auditProxy 写一条在线调试审计：只记方法、主机、路径与敏感变量名，不记查询/正文/值。
func (s *Service) auditProxy(ctx context.Context, userID uint64, req dto.ProxySendReq, table map[string]model.Variable, target *url.URL) {
	texts := []string{req.URL, req.Body}
	for _, kv := range req.Headers {
		texts = append(texts, kv.Name, kv.Value)
	}
	for _, kv := range req.Query {
		texts = append(texts, kv.Name, kv.Value)
	}
	for _, kv := range req.Form {
		texts = append(texts, kv.Name, kv.Value)
	}
	if req.Auth != nil {
		texts = append(texts, req.Auth.Username, req.Auth.Password, req.Auth.Token, req.Auth.Key, req.Auth.Value)
	}
	content := "在线调试 " + strings.ToUpper(req.Method) + " " + target.Scheme + "://" + target.Host + target.EscapedPath()
	if secrets := referencedSecretVars(texts, table); len(secrets) > 0 {
		content += "，使用敏感变量：" + strings.Join(secrets, ", ")
	}
	if len(content) > 500 {
		content = content[:500]
	}
	s.recordOp(ctx, req.ProjectID, userID, req.APIID, model.LogObjectAPI, "debug", content)
}

// proxyRowEnabled enabled 缺省视为启用。
func proxyRowEnabled(enabled *bool) bool { return enabled == nil || *enabled }

// isHopByHopHeader 过滤由 net/http 管理或会破坏代理语义的头。
func isHopByHopHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "host", "content-length", "transfer-encoding", "connection", "proxy-connection",
		"keep-alive", "te", "trailer", "upgrade":
		return true
	}
	return false
}

// basicAuthValue 生成 Basic 认证值。
func basicAuthValue(user, pass string) string {
	return base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
}

// mustProxyCIDRs 解析内置 CIDR；常量写错时启动即 panic。
func mustProxyCIDRs(cidrs ...string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, s := range cidrs {
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			panic("proxy: invalid CIDR " + s)
		}
		out = append(out, n)
	}
	return out
}
