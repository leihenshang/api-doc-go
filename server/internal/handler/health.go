package handler

import (
	"apidoc/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// Health 处理 GET /api/health，用作存活探针。
func (h *Handler) Health(c *gin.Context) {
	response.OK(c, gin.H{"status": "ok", "driver": "ready"})
}
