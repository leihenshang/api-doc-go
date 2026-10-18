package response

// P2-24：统一响应封装是「HTTP 恒 200 + 业务码在 body」这个全前端依赖契约的实现处。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"apidoc/internal/pkg/errcode"

	"github.com/gin-gonic/gin"
)

func TestOK(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	OK(c, map[string]any{"id": 1})

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP 状态应恒为 200，got %d", w.Code)
	}
	var body Body
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if body.Code != errcode.CodeSuccess || body.Msg != "success" {
		t.Fatalf("成功响应字段异常: %+v", body)
	}
	data, ok := body.Data.(map[string]any)
	if !ok || data["id"] != float64(1) {
		t.Fatalf("data 未正确透传: %#v", body.Data)
	}
}

func TestFail(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	// 显式文案优先
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	Fail(c, errcode.CodeParamError, "名称必填")

	var body Body
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if body.Code != errcode.CodeParamError || body.Msg != "名称必填" {
		t.Fatalf("失败响应应保留显式文案: %+v", body)
	}

	// 文案为空时取该码默认文案，而不是空串
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	Fail(c, errcode.CodeForbidden, "")
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if body.Msg != errcode.Message(errcode.CodeForbidden) || body.Msg == "" {
		t.Fatalf("空文案应回落到默认文案，got %q", body.Msg)
	}
	if body.Data != nil {
		t.Fatalf("失败响应 data 应为 null，got %#v", body.Data)
	}
}

// TestEnvelopeCodeHeader P2-7：HTTP 状态码保持 200（既有契约），但业务码同时写进
// `X-Api-Code` 头，监控/网关不必解析 body 就能统计失败率。
func TestEnvelopeCodeHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	OK(c, nil)
	if got := w.Header().Get("X-Api-Code"); got != strconv.Itoa(errcode.CodeSuccess) {
		t.Fatalf("成功响应头 X-Api-Code 应为 %d，got %q", errcode.CodeSuccess, got)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP 状态仍必须是 200，got %d", w.Code)
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	Fail(c, errcode.CodeForbidden, "")
	if got := w.Header().Get("X-Api-Code"); got != strconv.Itoa(errcode.CodeForbidden) {
		t.Fatalf("失败响应头 X-Api-Code 应为 %d，got %q", errcode.CodeForbidden, got)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("失败时 HTTP 状态仍必须是 200（前端按 body.code 分支），got %d", w.Code)
	}
}
