package handler

// HTTP 层单测：统一响应封装 + 鉴权中间件 + 登录/自助接口。
// 这一层此前零测试，而它承载了「HTTP 恒 200、错误码走 body」这个全前端依赖的契约 ——
// 一旦有人改成 4xx，前端 axios 会走异常分支拿不到结构化 msg。

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"apidoc/internal/config"
	"apidoc/internal/database"
	"apidoc/internal/dto"
	"apidoc/internal/middleware"
	"apidoc/internal/pkg/errcode"
	"apidoc/internal/pkg/response"
	"apidoc/internal/repository"
	"apidoc/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTestHandler(t *testing.T) (*Handler, *gin.Engine, *service.Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dsn := filepath.Join(t.TempDir(), "handler.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	cfg := &config.Config{}
	cfg.JWT.Secret = "handler-test-secret"
	cfg.JWT.ExpireHours = 1
	svc := service.New(repository.New(db, cfg), cfg)
	h := New(cfg, svc)

	// 只装配本测试要用的路由：不套限流，避免并发测试互相吃配额。
	engine := gin.New()
	engine.POST("/api/user/login", h.Login)
	auth := engine.Group("/api", middleware.AuthOrPAT(cfg.JWT.Secret, h.VerifyPAT))
	auth.GET("/user/me", h.Me)
	auth.GET("/user/list", h.ListUsers)
	return h, engine, svc
}

// call 发一次 JSON 请求并解出统一响应体。
func call(t *testing.T, engine *gin.Engine, method, path, token string, body any) response.Body {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	// 契约：HTTP 状态码恒为 200，业务码在 body 里。
	if w.Code != http.StatusOK {
		t.Fatalf("%s %s: HTTP 状态应为 200（统一响应契约），got %d", method, path, w.Code)
	}
	var out response.Body
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析响应失败: %v（body=%s）", err, w.Body.String())
	}
	return out
}

func TestUnauthenticatedRequestReturns34(t *testing.T) {
	_, engine, _ := newTestHandler(t)

	out := call(t, engine, http.MethodGet, "/api/user/me", "", nil)
	if out.Code != errcode.CodeUnauthorized {
		t.Fatalf("无凭证访问受保护接口应返回 %d，got %d (%s)", errcode.CodeUnauthorized, out.Code, out.Msg)
	}

	// 伪造 token 同样拒绝。
	out = call(t, engine, http.MethodGet, "/api/user/me", "not-a-jwt", nil)
	if out.Code != errcode.CodeUnauthorized {
		t.Fatalf("非法 token 应返回 %d，got %d", errcode.CodeUnauthorized, out.Code)
	}
}

func TestLoginThenMe(t *testing.T) {
	_, engine, svc := newTestHandler(t)
	if _, err := svc.InitAdmin(context.Background(), "admin", "admin123"); err != nil {
		t.Fatalf("InitAdmin: %v", err)
	}

	// 密码错误：业务码而不是 HTTP 4xx。
	out := call(t, engine, http.MethodPost, "/api/user/login", "", dto.LoginReq{Name: "admin", Pwd: "bad"})
	if out.Code == errcode.CodeSuccess {
		t.Fatalf("错误密码不应登录成功: %+v", out)
	}

	out = call(t, engine, http.MethodPost, "/api/user/login", "", dto.LoginReq{Name: "admin", Pwd: "admin123"})
	if out.Code != errcode.CodeSuccess {
		t.Fatalf("登录失败: %+v", out)
	}
	payload, ok := out.Data.(map[string]any)
	if !ok {
		t.Fatalf("登录响应 data 结构异常: %#v", out.Data)
	}
	token, _ := payload["token"].(string)
	if token == "" {
		t.Fatalf("登录响应缺少 token: %#v", payload)
	}

	out = call(t, engine, http.MethodGet, "/api/user/me", token, nil)
	if out.Code != errcode.CodeSuccess {
		t.Fatalf("带 token 取 /user/me 失败: %+v", out)
	}
	me, ok := out.Data.(map[string]any)
	if !ok || me["name"] != "admin" {
		t.Fatalf("/user/me 返回内容异常: %#v", out.Data)
	}
}

func TestListUsersNonAdminScrubsPrivateFields(t *testing.T) {
	_, engine, svc := newTestHandler(t)
	ctx := context.Background()
	if _, err := svc.InitAdmin(ctx, "admin", "admin123"); err != nil {
		t.Fatalf("InitAdmin: %v", err)
	}
	if _, err := svc.Register(ctx, dto.RegisterReq{
		Name: "bob", Pwd: "p123456", RePwd: "p123456", Email: "bob@example.com",
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	out := call(t, engine, http.MethodPost, "/api/user/login", "", dto.LoginReq{Name: "bob", Pwd: "p123456"})
	token, _ := out.Data.(map[string]any)["token"].(string)
	if token == "" {
		t.Fatalf("bob 登录失败: %+v", out)
	}

	out = call(t, engine, http.MethodGet, "/api/user/list", token, nil)
	if out.Code != errcode.CodeSuccess {
		t.Fatalf("普通用户拉用户列表应成功（仅脱敏）: %+v", out)
	}
	raw, err := json.Marshal(out.Data)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if bytes.Contains(raw, []byte("bob@example.com")) {
		t.Fatalf("普通用户不应看到他人邮箱: %s", raw)
	}
	if !bytes.Contains(raw, []byte("bob")) {
		t.Fatalf("id→名称映射仍需要用户名，不能整体拒绝: %s", raw)
	}
}
