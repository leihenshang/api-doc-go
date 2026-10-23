package handler

// 在线调试 handler 的最小契约测试：参数绑定失败走统一响应（HTTP 恒 200 + code 22）。
// SSRF/变量渲染等业务行为在 service 层测试，这里只锁 HTTP 层。

import (
	"net/http"
	"testing"

	"apidoc/internal/pkg/errcode"
)

func TestProxySendParamError(t *testing.T) {
	t.Parallel()
	h, engine, _ := newTestHandler(t)
	engine.POST("/proxy", h.ProxySend)

	out := call(t, engine, http.MethodPost, "/proxy", "", map[string]any{"project_id": 1})
	if out.Code != errcode.CodeParamError {
		t.Fatalf("缺少 method/url 应返回 %d，got %d (%s)", errcode.CodeParamError, out.Code, out.Msg)
	}
}
