package middleware

import (
	"strings"

	"apidoc/internal/pkg/errcode"
	"apidoc/internal/pkg/jwt"
	"apidoc/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// 上下文键。
const (
	CtxPrincipal = "principal"
)

// Principal 是当前登录主体。
type Principal struct {
	UserID uint64
	Type   int8
}

// Auth 校验 JWT（优先 Authorization: Bearer，兼容 query/body 的 token 参数）。
// 校验失败写回 code=34 并中断。secret 为空将拒绝所有请求（应尽早暴露配置问题）。
func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tok := bearer(c.GetHeader("Authorization"))
		if tok == "" {
			tok = c.Query("token")
			if tok == "" {
				tok = c.PostForm("token")
			}
		}
		claims, err := jwt.Parse(tok, secret)
		if err != nil {
			response.Fail(c, errcode.CodeUnauthorized, "")
			c.Abort()
			return
		}
		c.Set(CtxPrincipal, Principal{UserID: claims.UserID, Type: claims.Type})
		c.Next()
	}
}

// Current 取当前主体；未经过 Auth 中间件时返回 false。
func Current(c *gin.Context) (Principal, bool) {
	v, ok := c.Get(CtxPrincipal)
	if !ok {
		return Principal{}, false
	}
	p, ok := v.(Principal)
	return p, ok
}

// bearer 从 "Bearer xxx" 中提取令牌，非该前缀返回空串。
func bearer(h string) string {
	const prefix = "Bearer "
	if len(h) >= len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}
