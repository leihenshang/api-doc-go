package handler

// 契约校验（RM-08）：OpenAPI 与项目当前接口的只读 diff。

import (
	"net/http"

	"apidoc/internal/dto"
	"apidoc/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// ContractDiff 处理 POST /api/v1/item/contract-diff（仅 v1）。
func (h *Handler) ContractDiff(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
	var req dto.ContractDiffReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.ContractDiff(c.Request.Context(), p.UserID, p.Type, req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}
