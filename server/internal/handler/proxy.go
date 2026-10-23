package handler

import (
	"net/http"

	"apidoc/internal/dto"
	"apidoc/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// maxProxyRequestBytes 限制在线调试请求体大小，避免把文档服务当大文件上传通道。
const maxProxyRequestBytes = 2 << 20

// ProxySend 处理 POST /api/v1/proxy/send（在线调试；项目读权限）。
// 只挂在 v1 前缀：桌面客户端不需要该能力，历史前缀不再新增路由（P2-27）。
func (h *Handler) ProxySend(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxProxyRequestBytes)
	var req dto.ProxySendReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.ProxySend(c.Request.Context(), p.UserID, p.Type, req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}
