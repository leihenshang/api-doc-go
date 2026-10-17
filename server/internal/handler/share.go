// 分享链接（受保护：创建/列表/删除）+ 免登录只读分享数据 + Mock 服务。
package handler

import (
	"net/http"
	"strconv"
	"strings"

	"apidoc/internal/dto"
	"apidoc/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// CreateShareLink 处理 POST /api/share，生成免登录只读链接。
func (h *Handler) CreateShareLink(c *gin.Context) {
	var req dto.ShareLinkReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.CreateShareLink(c.Request.Context(), p.UserID, p.Type, req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// ListShareLinks 处理 GET /api/share?project_id=（需登录 + 项目读权限；令牌脱敏）。
func (h *Handler) ListShareLinks(c *gin.Context) {
	p := principal(c)
	res, err := h.svc.ListShareLinks(c.Request.Context(), p.UserID, p.Type, idQuery(c, "project_id"))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// DeleteShareLink 处理 DELETE /api/share?id=，删除即失效。
func (h *Handler) DeleteShareLink(c *gin.Context) {
	p := principal(c)
	if err := h.svc.DeleteShareLink(c.Request.Context(), p.UserID, p.Type, idParam(c)); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, nil)
}

// SharePayload 处理 GET /api/share/:token（**免登录**），返回只读的项目文档数据。
func (h *Handler) SharePayload(c *gin.Context) {
	res, err := h.svc.SharePayload(c.Request.Context(), c.Param("token"))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// Mock 处理 ANY /mock/:project_id/*path（**免登录**）：按方法 + 路径匹配接口并返回响应样例。
// 出错时按 HTTP 404 返回 JSON，便于前端直接联调。
func (h *Handler) Mock(c *gin.Context) {
	pid, _ := strconv.ParseUint(c.Param("project_id"), 10, 64)
	res, err := h.svc.MockResponse(c.Request.Context(), pid, c.Request.Method, c.Param("path"))
	if err != nil {
		body := `{"code":404,"msg":"` + strings.ReplaceAll(err.Error(), `"`, "'") + `"}`
		c.Data(http.StatusNotFound, "application/json; charset=utf-8", []byte(body))
		return
	}
	c.Data(res.Status, res.ContentType, []byte(res.Body))
}
