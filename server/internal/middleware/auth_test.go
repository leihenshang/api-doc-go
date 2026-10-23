package middleware

// 鉴权/CORS 回归：JWT/PAT/refresh 回源、Current、CORS 预检。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
	"apidoc/internal/pkg/jwt"

	"github.com/gin-gonic/gin"
)

func TestAuthJWTAndCurrent(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	engine := newAuthEngine(Auth("secret"))
	tok, err := jwt.Generate(42, model.UserTypeSuperuser, jwt.Issuer, "secret", 1)
	if err != nil {
		t.Fatalf("jwt.Generate: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("JWT 应通过，got %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Code int `json:"code"`
		Data struct {
			UID  uint64 `json:"uid"`
			Type int8   `json:"type"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || out.Code != 200 || out.Data.UID != 42 || out.Data.Type != model.UserTypeSuperuser {
		t.Fatalf("Current 未取到主体: %s (%v)", w.Body.String(), err)
	}
}

func TestAuthRejectsMissingToken(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	engine := newAuthEngine(Auth("secret"))
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("鉴权失败也应 HTTP 200，got %d", w.Code)
	}
	var out struct {
		Code int `json:"code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out.Code != errcode.CodeUnauthorized {
		t.Fatalf("缺 token 应返回 code 34，got %d %s", out.Code, w.Body.String())
	}
}

func TestAuthOrPATWithRefresh(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	refreshOK := func(_ *gin.Context, _ uint64) (int8, bool) { return model.UserTypeNormal, true }
	engine := newAuthEngine(AuthOrPATWithRefresh("secret", func(raw string) (uint64, int8, error) {
		if raw == "pat_good" {
			return 7, model.UserTypeNormal, nil
		}
		return 0, 0, errInvalidToken{}
	}, refreshOK))

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer pat_good")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	var out struct {
		Code int `json:"code"`
		Data struct {
			UID uint64 `json:"uid"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || out.Code != 200 || out.Data.UID != 7 {
		t.Fatalf("PAT 应通过: %s (%v)", w.Body.String(), err)
	}

	// refresh 返回不可用时即便 JWT 有效也必须拒绝。
	engine = newAuthEngine(AuthOrPATWithRefresh("secret", nil, func(_ *gin.Context, _ uint64) (int8, bool) {
		return 0, false
	}))
	tok, _ := jwt.Generate(42, model.UserTypeSuperuser, jwt.Issuer, "secret", 1)
	req = httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out.Code != errcode.CodeUnauthorized {
		t.Fatalf("refresh 拒绝应返回 34，got %s", w.Body.String())
	}
}

func TestCORS(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	engine := newAuthEngine(CORS(), func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	req := httptest.NewRequest(http.MethodOptions, "/me", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent || w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("CORS 预检失败: %d %v", w.Code, w.Header())
	}
}

// newAuthEngine 构造带中间件与 /me 的测试引擎。
func newAuthEngine(mw ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(mw...)
	e.GET("/me", func(c *gin.Context) {
		p, ok := Current(c)
		if !ok {
			c.JSON(http.StatusOK, gin.H{"code": errcode.CodeUnauthorized})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"uid": p.UserID, "type": p.Type}})
	})
	return e
}

type errInvalidToken struct{}

func (errInvalidToken) Error() string { return "invalid token" }
