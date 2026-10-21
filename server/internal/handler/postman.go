// Postman 集合导入导出 handler。
package handler

import (
	"io"
	"net/http"

	"apidoc/internal/pkg/errcode"
	"apidoc/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// ExportPostman 处理 GET /api/api/export-postman?project_id=&group_id=，返回 Postman Collection v2.1 JSON。
func (h *Handler) ExportPostman(c *gin.Context) {
	p := principal(c)
	res, err := h.svc.ExportPostman(c.Request.Context(), p.UserID, p.Type, idQuery(c, "project_id"), idQuery(c, "group_id"))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// ImportPostman 处理 POST /api/api/import-postman?project_id=&strategy=，请求体为 Postman 集合 JSON。
func (h *Handler) ImportPostman(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImportBodyBytes)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil || len(body) == 0 {
		response.Fail(c, errcode.CodeParamError, "请求体为空")
		return
	}
	p := principal(c)
	res, err := h.svc.ImportPostman(c.Request.Context(), p.UserID, p.Type, idQuery(c, "project_id"), c.Query("strategy"), body)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}
