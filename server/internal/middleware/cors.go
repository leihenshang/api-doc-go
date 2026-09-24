// Package middleware 提供跨模块复用的 gin 中间件。
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS 放行跨域请求（开发期宽松策略，生产应收敛为白名单）。
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin,Content-Type,Authorization,token")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
