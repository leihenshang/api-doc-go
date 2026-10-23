package handler

// 接口/文档历史快照（RM-05）：列表、字段级 diff、回滚。

import (
	"strconv"
	"strings"

	"apidoc/internal/dto"
	"apidoc/internal/pkg/errcode"
	"apidoc/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// ListRevisions 处理 GET /api/v1/item/revisions?type=api|doc&id=。
func (h *Handler) ListRevisions(c *gin.Context) {
	p := principal(c)
	res, err := h.svc.ListRevisions(c.Request.Context(), p.UserID, p.Type, revisionType(c), idQuery(c, "id"))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// RevisionDiff 处理 GET /api/v1/item/revision-diff?type=api|doc&id=&revision_id=。
func (h *Handler) RevisionDiff(c *gin.Context) {
	p := principal(c)
	revisionID, _ := strconv.ParseUint(c.Query("revision_id"), 10, 64)
	if revisionID == 0 {
		response.Fail(c, errcode.CodeParamError, "revision_id 必填")
		return
	}
	res, err := h.svc.RevisionDiff(c.Request.Context(), p.UserID, p.Type, revisionType(c), idQuery(c, "id"), revisionID)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// RestoreRevision 处理 POST /api/v1/item/revision-restore（body type/id/revision_id）。
func (h *Handler) RestoreRevision(c *gin.Context) {
	var req dto.RevisionRestoreReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	if err := h.svc.RestoreRevision(c.Request.Context(), p.UserID, p.Type, strings.ToLower(strings.TrimSpace(req.Type)), req.ID, req.RevisionID); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, nil)
}

// revisionType 归一化查询参数（api/doc）；非法值交给 service 统一报 22。
func revisionType(c *gin.Context) string {
	return strings.ToLower(strings.TrimSpace(c.Query("type")))
}
