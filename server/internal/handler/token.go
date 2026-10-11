package handler

import (
	"context"

	"apidoc/internal/pkg/errcode"
	"apidoc/internal/pkg/response"
	"apidoc/internal/service"

	"github.com/gin-gonic/gin"
)

// CreateToken POST /api/user/token
func (h *Handler) CreateToken(c *gin.Context) {
	var req service.CreateTokenReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	out, err := h.svc.CreateToken(c.Request.Context(), p.UserID, &req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, out)
}

// ListTokens GET /api/user/token
func (h *Handler) ListTokens(c *gin.Context) {
	p := principal(c)
	out, err := h.svc.ListTokens(c.Request.Context(), p.UserID)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, out)
}

// RevokeToken DELETE /api/user/token?id=（删除，不可恢复）
func (h *Handler) RevokeToken(c *gin.Context) {
	id := idParam(c)
	if id == 0 {
		response.Fail(c, errcode.CodeParamError, "id 必填")
		return
	}
	p := principal(c)
	if err := h.svc.DeleteToken(c.Request.Context(), p.UserID, id); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, nil)
}

// RevealToken GET /api/user/token/reveal?id=（查看本人 PAT 明文，服务端解密）
func (h *Handler) RevealToken(c *gin.Context) {
	id := idParam(c)
	if id == 0 {
		response.Fail(c, errcode.CodeParamError, "id 必填")
		return
	}
	p := principal(c)
	raw, err := h.svc.RevealToken(c.Request.Context(), p.UserID, id)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, gin.H{"token": raw})
}

// VerifyPAT 供鉴权中间件回调（S7）。
func (h *Handler) VerifyPAT(raw string) (uint64, error) {
	return h.svc.VerifyToken(context.Background(), raw)
}
