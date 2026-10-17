package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"apidoc/internal/pkg/errcode"

	"github.com/gin-gonic/gin"
)

// newLimitEngine 构造带限流的测试引擎（固定返回 200 的 /ping）。
func newLimitEngine(limit int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RateLimit(limit))
	engine.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })
	return engine
}

// doPing 以指定 IP 发一次请求，返回状态码与响应体。
func doPing(engine *gin.Engine, ip string) (int, string) {
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = ip + ":1234"
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

// TestRateLimitPerIP 覆盖限流：超限返回 HTTP 200 + code 14 且带 Retry-After；不同 IP 互不影响。
func TestRateLimitPerIP(t *testing.T) {
	engine := newLimitEngine(2)

	if code, body := doPing(engine, "1.1.1.1"); code != http.StatusOK || body != "pong" {
		t.Fatalf("第 1 次应放行，got %d %s", code, body)
	}
	if _, body := doPing(engine, "1.1.1.1"); body != "pong" {
		t.Fatalf("第 2 次应放行，got %s", body)
	}

	code, body := doPing(engine, "1.1.1.1")
	if code != http.StatusOK {
		t.Fatalf("限流命中仍应返回 HTTP 200（前端只认 body.code），got %d", code)
	}
	var res struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal([]byte(body), &res); err != nil {
		t.Fatalf("响应不是统一结构：%s", body)
	}
	if res.Code != errcode.CodeFailed || res.Msg == "" {
		t.Fatalf("限流命中应返回业务失败码与提示，got %+v", res)
	}

	// 其它 IP 不受影响。
	if _, body := doPing(engine, "2.2.2.2"); body != "pong" {
		t.Fatalf("不同 IP 应独立计数，got %s", body)
	}
}

// TestRateLimitDisabledAndRetryAfter 覆盖关闭开关与 Retry-After 头。
func TestRateLimitDisabled(t *testing.T) {
	engine := newLimitEngine(0)
	for i := 0; i < 5; i++ {
		if _, body := doPing(engine, "3.3.3.3"); body != "pong" {
			t.Fatalf("limit=0 表示关闭限流，第 %d 次被拦：%s", i+1, body)
		}
	}
}

// TestRateLimitRetryAfter 覆盖 Retry-After 头存在且为正整数。
func TestRateLimitRetryAfter(t *testing.T) {
	engine := newLimitEngine(1)
	doPing(engine, "4.4.4.4")
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = "4.4.4.4:1234"
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	ra := w.Header().Get("Retry-After")
	n, err := strconv.Atoi(ra)
	if err != nil || n < 1 || n > 60 {
		t.Fatalf("Retry-After 应为 1..60 的整数，got %q", ra)
	}
}

// TestClientIP 覆盖客户端 IP 解析：优先 X-Forwarded-For / X-Real-IP。
func TestClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.RemoteAddr = "10.0.0.1:9"
	if ip := ClientIP(c); ip != "10.0.0.1" {
		t.Fatalf("应回退 RemoteAddr，got %q", ip)
	}
	c.Request.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
	if ip := ClientIP(c); ip != "203.0.113.7" {
		t.Fatalf("应取 XFF 首个地址，got %q", ip)
	}
	c.Request.Header.Del("X-Forwarded-For")
	c.Request.Header.Set("X-Real-IP", "198.51.100.9")
	if ip := ClientIP(c); ip != "198.51.100.9" {
		t.Fatalf("应取 X-Real-IP，got %q", ip)
	}
}

// TestRateLimiterBoundedEntries P1-11：X-Forwarded-For 是客户端可控的，
// 大量伪造 IP 不能在窗口内把计数器 map 撑到无界。
func TestRateLimiterBoundedEntries(t *testing.T) {
	l := newRateLimiter(5)
	l.maxEntries = 100
	for i := 0; i < 10000; i++ {
		l.allow("10.0." + strconv.Itoa(i/256) + "." + strconv.Itoa(i%256))
	}
	if got := len(l.hits); got > l.maxEntries+1 { // +1 = 溢出桶
		t.Fatalf("计数器条目数应被上限约束，got %d（上限 %d）", got, l.maxEntries)
	}
	// 溢出桶也要真的限流：否则「换 IP」等于绕过限流。
	blocked := false
	for i := 0; i < 50; i++ {
		if !l.allow("172.16.0." + strconv.Itoa(i)) {
			blocked = true
			break
		}
	}
	if !blocked {
		t.Fatal("达到条目上限后新 IP 必须落进溢出桶并受限")
	}
}

// TestRateLimiterWindowReset 窗口过期后重新放行，且过期的键会被清理。
func TestRateLimiterWindowReset(t *testing.T) {
	l := newRateLimiter(2)
	now := time.Now()
	l.nowFunc = func() time.Time { return now }
	if !l.allow("1.1.1.1") || !l.allow("1.1.1.1") {
		t.Fatal("窗口内前两次应放行")
	}
	if l.allow("1.1.1.1") {
		t.Fatal("窗口内第三次应被限流")
	}
	// 推进到下一窗口：应当重新放行，并顺带 GC 掉旧窗口
	now = now.Add(2 * time.Minute)
	if !l.allow("1.1.1.1") {
		t.Fatal("新窗口应重新放行")
	}
	if got := len(l.hits); got != 1 {
		t.Fatalf("过期窗口应被清理，got %d 条", got)
	}
}

// TestRateLimiterDisabled limit<=0 时一律放行。
func TestRateLimiterDisabled(t *testing.T) {
	l := newRateLimiter(0)
	for i := 0; i < 100; i++ {
		if !l.allow("any") {
			t.Fatal("limit<=0 应不限制")
		}
	}
}
