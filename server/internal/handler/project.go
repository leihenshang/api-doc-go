package handler

import (
	"apidoc/internal/dto"
	"apidoc/internal/pkg/page"
	"apidoc/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// CreateProject 处理 POST /api/project。
func (h *Handler) CreateProject(c *gin.Context) {
	var req dto.ProjectCreateReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.CreateProject(c.Request.Context(), p.UserID, req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// ListProjects 处理 GET /api/project。
func (h *Handler) ListProjects(c *gin.Context) {
	var q page.Query
	_ = c.ShouldBindQuery(&q)
	p := principal(c)
	res, err := h.svc.ListProjects(c.Request.Context(), p.UserID, p.Type, q)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// GetProject 处理 GET /api/project/:id。
func (h *Handler) GetProject(c *gin.Context) {
	res, err := h.svc.GetProject(c.Request.Context(), idParam(c))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// UpdateProject 处理 PUT /api/project。
func (h *Handler) UpdateProject(c *gin.Context) {
	var req dto.ProjectUpdateReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.UpdateProject(c.Request.Context(), p.UserID, p.Type, req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// DeleteProject 处理 DELETE /api/project/:id。
func (h *Handler) DeleteProject(c *gin.Context) {
	p := principal(c)
	if err := h.svc.DeleteProject(c.Request.Context(), p.UserID, p.Type, idParam(c)); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, nil)
}

// AddMember 处理 POST /api/project/member。
func (h *Handler) AddMember(c *gin.Context) {
	var req dto.MemberAddReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	if err := h.svc.AddMember(c.Request.Context(), p.UserID, p.Type, req); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, nil)
}

// ListMembers 处理 GET /api/project/members。
func (h *Handler) ListMembers(c *gin.Context) {
	res, err := h.svc.ListMembers(c.Request.Context(), idOrFallback(c, "project_id"))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// SetLeader 处理 PUT /api/project/leader。
func (h *Handler) SetLeader(c *gin.Context) {
	var req dto.SetLeaderReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	if err := h.svc.SetLeader(c.Request.Context(), p.UserID, p.Type, req); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, nil)
}

// SetPermission 处理 PUT /api/project/permission。
func (h *Handler) SetPermission(c *gin.Context) {
	var req dto.SetPermissionReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	if err := h.svc.SetPermission(c.Request.Context(), p.UserID, p.Type, req); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, nil)
}

// QuitProject 处理 POST /api/project/quit。
func (h *Handler) QuitProject(c *gin.Context) {
	var req dto.QuitProjectReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	if err := h.svc.QuitProject(c.Request.Context(), p.UserID, req); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, nil)
}

// MyPermission 处理 GET /api/project/permission，返回当前用户在项目中的权限掩码。
func (h *Handler) MyPermission(c *gin.Context) {
	p := principal(c)
	perm := h.svc.MyPermission(c.Request.Context(), p.UserID, p.Type, idQuery(c, "project_id"))
	response.OK(c, gin.H{"permission": perm})
}
