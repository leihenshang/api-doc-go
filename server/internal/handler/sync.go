package handler

import (
	"strconv"

	"apidoc/internal/pkg/errcode"
	"apidoc/internal/pkg/response"
	"apidoc/internal/service"

	"github.com/gin-gonic/gin"
)

// Meta 能力协商 GET /api/meta（免登录亦可，挂在受保护组内 + 公共组）。
func (h *Handler) Meta(c *gin.Context) {
	response.OK(c, h.svc.Meta(c.Request.Context()))
}

// SyncState GET /api/sync/state?project_id=
func (h *Handler) SyncState(c *gin.Context) {
	pid, _ := strconv.ParseUint(c.Query("project_id"), 10, 64)
	if pid == 0 {
		response.Fail(c, errcode.CodeParamError, "project_id 必填")
		return
	}
	out, err := h.svc.SyncState(c.Request.Context(), pid)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, out)
}

// SyncChanges GET /api/sync/changes?project_id=&since=&limit=
func (h *Handler) SyncChanges(c *gin.Context) {
	pid, _ := strconv.ParseUint(c.Query("project_id"), 10, 64)
	if pid == 0 {
		response.Fail(c, errcode.CodeParamError, "project_id 必填")
		return
	}
	since, _ := strconv.ParseInt(c.Query("since"), 10, 64)
	limit, _ := strconv.Atoi(c.Query("limit"))
	out, err := h.svc.SyncChanges(c.Request.Context(), pid, since, limit)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, out)
}

// SyncSnapshot GET /api/sync/snapshot?project_id=&offset=&limit=
func (h *Handler) SyncSnapshot(c *gin.Context) {
	pid, _ := strconv.ParseUint(c.Query("project_id"), 10, 64)
	if pid == 0 {
		response.Fail(c, errcode.CodeParamError, "project_id 必填")
		return
	}
	offset, _ := strconv.Atoi(c.Query("offset"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	out, err := h.svc.SyncSnapshot(c.Request.Context(), pid, offset, limit)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, out)
}

// SyncPush POST /api/sync/push
func (h *Handler) SyncPush(c *gin.Context) {
	var req service.SyncPushReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	out, err := h.svc.SyncPush(c.Request.Context(), p.UserID, p.Type, &req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, out)
}
