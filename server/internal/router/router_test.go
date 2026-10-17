package router

// P2-22：服务端此前没有任何「进程级」集成测试 —— handler 层单测是手工装配最小路由，
// 中间件链路（Recovery → CORS → 限流 → AuthOrPAT）、路由注册、统一响应封装、
// 真实 service + 真实 SQLite 的组合从未被一起跑过。
//
// 这里用 router.Setup 起真实引擎，用 httptest 走完整 HTTP 链路：
// 登录 → 建项目 → 建分组/接口 → 同步 push/pull → 分享链接 → 回收站还原。

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"apidoc/internal/config"
	"apidoc/internal/database"
	"apidoc/internal/pkg/errcode"
	"apidoc/internal/pkg/response"
	"apidoc/internal/repository"
	"apidoc/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newTestEngine 起一个与生产同构的引擎（真实路由 + 真实 service + 内存前的临时 SQLite）。
func newTestEngine(t *testing.T) (*gin.Engine, *service.Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dsn := filepath.Join(t.TempDir(), "router.db")
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
	cfg.JWT.Secret = "router-test-secret"
	cfg.JWT.ExpireHours = 1
	// 限流关掉：集成测试里多次请求同一 IP，开着会给测试引入与逻辑无关的 14 码
	cfg.Security.RateLimitPerMin = 0
	cfg.Security.AuthRateLimitPerMin = 0
	engine := Setup(db, cfg)
	return engine, service.New(repository.New(db, cfg), cfg)
}

// do 发一次请求；返回解包后的统一响应体与 HTTP 状态码。
func do(t *testing.T, e *gin.Engine, method, path, token string, body any) (response.Body, int) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	var out response.Body
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("%s %s 响应不是统一结构: %v（body=%s）", method, path, err, w.Body.String())
	}
	return out, w.Code
}

// mustOK 断言业务码为 200 并返回 data。
func mustOK(t *testing.T, out response.Body, code int, what string) any {
	t.Helper()
	if code != http.StatusOK {
		t.Fatalf("%s: HTTP 应恒为 200，got %d", what, code)
	}
	if out.Code != errcode.CodeSuccess {
		t.Fatalf("%s 失败: code=%d msg=%s", what, out.Code, out.Msg)
	}
	return out.Data
}

// TestFullStackFlow 全链路：登录 → 建项目 → 建分组/接口 → 同步 → 分享 → 回收站。
func TestFullStackFlow(t *testing.T) {
	t.Parallel()
	engine, svc := newTestEngine(t)
	ctx := t.Context()
	if _, err := svc.InitAdmin(ctx, "admin", "admin123"); err != nil {
		t.Fatalf("InitAdmin: %v", err)
	}

	// 1. 登录（公共接口 + 统一响应）
	out, code := do(t, engine, http.MethodPost, "/api/user/login", "", map[string]any{"name": "admin", "pwd": "admin123"})
	loginData := mustOK(t, out, code, "登录").(map[string]any)
	token, _ := loginData["token"].(string)
	if token == "" {
		t.Fatal("登录未返回 token")
	}

	// 2. 建项目
	out, code = do(t, engine, http.MethodPost, "/api/project", token, map[string]any{"title": "集成测试项目"})
	proj := mustOK(t, out, code, "建项目").(map[string]any)
	projID := uint64(proj["id"].(float64))

	// 3. 建分组与接口
	out, code = do(t, engine, http.MethodPost, "/api/group", token, map[string]any{
		"project_id": projID, "title": "用户", "type": 1,
	})
	group := mustOK(t, out, code, "建分组").(map[string]any)
	groupID := uint64(group["id"].(float64))

	out, code = do(t, engine, http.MethodPost, "/api/api", token, map[string]any{
		"project_id": projID, "group_id": groupID, "api_name": "列表",
		"http_method_type": "GET", "url": "/v1/user",
	})
	api := mustOK(t, out, code, "建接口").(map[string]any)
	apiID := uint64(api["id"].(float64))
	if api["ext_uid"] == nil || api["ext_uid"] == "" {
		t.Fatal("新建接口必须带 ext_uid（同步要用）")
	}

	// 4. 列表读回来（走真实鉴权 + 项目权限）
	out, code = do(t, engine, http.MethodGet, "/api/api?project_id=1&page_num=1&page_size=10", token, nil)
	mustOK(t, out, code, "接口列表")

	// 5. 同步：状态 / 快照 / 推送
	out, code = do(t, engine, http.MethodGet, "/api/sync/state?project_id=1", token, nil)
	mustOK(t, out, code, "同步状态")

	out, code = do(t, engine, http.MethodGet, "/api/sync/snapshot?project_id=1&offset=0&limit=50", token, nil)
	snap := mustOK(t, out, code, "同步快照").(map[string]any)
	if items, _ := snap["items"].([]any); len(items) == 0 {
		t.Fatal("快照应包含已建的接口/分组")
	}

	payload, _ := json.Marshal(service.SyncPayload{Title: "推送的分组"})
	out, code = do(t, engine, http.MethodPost, "/api/sync/push", token, map[string]any{
		"project_id": 1, "device_id": "it-dev",
		"ops": []map[string]any{{
			"op_id": "it-op-1", "op": "upsert", "type": "group",
			"uid": "it-uid-1", "base_rev": 0, "payload": json.RawMessage(payload),
		}},
	})
	push := mustOK(t, out, code, "同步推送").(map[string]any)
	results, _ := push["results"].([]any)
	if len(results) != 1 || results[0].(map[string]any)["status"] != "ok" {
		t.Fatalf("推送应成功: %+v", push)
	}

	// 6. 分享链接（免登录可读）
	out, code = do(t, engine, http.MethodPost, "/api/share", token, map[string]any{"project_id": 1})
	share := mustOK(t, out, code, "建分享").(map[string]any)
	shareToken, _ := share["token"].(string)
	if shareToken == "" {
		t.Fatalf("分享未返回 token: %+v", share)
	}
	out, code = do(t, engine, http.MethodGet, "/api/share/"+shareToken, "", nil)
	mustOK(t, out, code, "匿名读分享")

	// 7. 删除进回收站 → 回收站列表 → 还原
	out, code = do(t, engine, http.MethodDelete, "/api/api?id="+itoa(apiID), token, nil)
	mustOK(t, out, code, "删除接口")

	out, code = do(t, engine, http.MethodGet, "/api/api?project_id=1&deleted=1", token, nil)
	trash := mustOK(t, out, code, "回收站列表").(map[string]any)
	if items, _ := trash["items"].([]any); len(items) != 1 {
		t.Fatalf("回收站应有 1 条，got %+v", trash)
	}

	out, code = do(t, engine, http.MethodPost, "/api/api/restore", token, map[string]any{"id": apiID})
	mustOK(t, out, code, "还原接口")

	out, code = do(t, engine, http.MethodGet, "/api/api?project_id=1", token, nil)
	live := mustOK(t, out, code, "还原后列表").(map[string]any)
	if items, _ := live["items"].([]any); len(items) != 1 {
		t.Fatalf("还原后应有 1 条活数据，got %+v", live)
	}

	// 8. 越权：另一个用户读同一个项目要被拒
	out, code = do(t, engine, http.MethodPost, "/api/user/register", "", map[string]any{
		"name": "bob", "pwd": "p123456", "re_pwd": "p123456",
	})
	mustOK(t, out, code, "注册 bob")
	out, code = do(t, engine, http.MethodPost, "/api/user/login", "", map[string]any{"name": "bob", "pwd": "p123456"})
	bobToken, _ := mustOK(t, out, code, "bob 登录").(map[string]any)["token"].(string)
	out, _ = do(t, engine, http.MethodGet, "/api/api?project_id=1", bobToken, nil)
	if out.Code != errcode.CodeForbidden {
		t.Fatalf("非成员读他人项目应 403，got code=%d msg=%s", out.Code, out.Msg)
	}
	if groupID == 0 || apiID == 0 {
		t.Fatal("自检：分组/接口 id 不应为 0")
	}
}

// itoa 避免为一行引入 strconv 依赖歧义。
func itoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
