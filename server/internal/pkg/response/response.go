// Package response 统一封装 JSON 响应为 {code,msg,data}，HTTP 状态码恒为 200。
package response

import (
	"net/http"
	"strconv"

	"apidoc/internal/pkg/errcode"

	"github.com/gin-gonic/gin"
)

// headerCode 把业务码同时写进响应头（P2-7）。
//
// HTTP 状态码恒为 200 是既有对外契约（前端 axios 只解包 body、网关按 200 放行），
// 直接改成 4xx/5xx 会让两端同时失效。但「监控/网关看不到失败率」是真问题，
// 因此在头里暴露机器可读的业务码：`X-Api-Code: 34`。采集侧按头聚合即可，
// 不需要解析 body；将来要上真正的 HTTP 语义，走 `/api/v2` 再改。
const headerCode = "X-Api-Code"

// Body 是统一响应体。
type Body struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

// OK 写出成功响应，携带数据 data（可为 nil）。
func OK(c *gin.Context, data any) {
	c.Header(headerCode, strconv.Itoa(errcode.CodeSuccess))
	c.JSON(http.StatusOK, Body{Code: errcode.CodeSuccess, Msg: errcode.Message(errcode.CodeSuccess), Data: data})
}

// Fail 写出失败响应，使用给定错误码；msg 为空时取该码默认文案。
func Fail(c *gin.Context, code int, msg string) {
	if msg == "" {
		msg = errcode.Message(code)
	}
	c.Header(headerCode, strconv.Itoa(code))
	c.JSON(http.StatusOK, Body{Code: code, Msg: msg, Data: nil})
}
