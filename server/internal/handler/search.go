package handler

import (
	"strconv"

	"apidoc/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// Search 处理 GET /api/search?keyword=&limit=，跨项目搜索接口 / 文档 / 项目。
func (h *Handler) Search(c *gin.Context) {
	p := principal(c)
	limit, _ := strconv.Atoi(c.Query("limit"))
	res, err := h.svc.Search(c.Request.Context(), p.UserID, p.Type, c.Query("keyword"), limit)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}
