// Package response 统一封装 JSON 响应为 {code,msg,data}，HTTP 状态码恒为 200。
package response

import (
	"net/http"

	"apidoc/internal/pkg/errcode"

	"github.com/gin-gonic/gin"
)

// Body 是统一响应体。
type Body struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

// OK 写出成功响应，携带数据 data（可为 nil）。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{Code: errcode.CodeSuccess, Msg: errcode.Message(errcode.CodeSuccess), Data: data})
}

// Fail 写出失败响应，使用给定错误码；msg 为空时取该码默认文案。
func Fail(c *gin.Context, code int, msg string) {
	if msg == "" {
		msg = errcode.Message(code)
	}
	c.JSON(http.StatusOK, Body{Code: code, Msg: msg, Data: nil})
}
