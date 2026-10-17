package handler

import (
	"strconv"

	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/pkg/page"
	"apidoc/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// Login 处理 POST /api/user/login。
func (h *Handler) Login(c *gin.Context) {
	var req dto.LoginReq
	if !bindJSON(c, &req) {
		return
	}
	res, err := h.svc.Login(c.Request.Context(), req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// Register 处理 POST /api/user/register。
func (h *Handler) Register(c *gin.Context) {
	var req dto.RegisterReq
	if !bindJSON(c, &req) {
		return
	}
	u, err := h.svc.Register(c.Request.Context(), req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, u)
}

// Me 处理 GET /api/user/me，返回当前用户。
func (h *Handler) Me(c *gin.Context) {
	p := principal(c)
	u, err := h.svc.GetUser(c.Request.Context(), p.UserID)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, u)
}

// ListUsers 处理 GET /api/user/list。
func (h *Handler) ListUsers(c *gin.Context) {
	var q page.Query
	_ = c.ShouldBindQuery(&q)
	p := principal(c)
	operator := model.UserInfo{BaseModel: model.BaseModel{ID: p.UserID}, Type: p.Type}
	items, err := h.svc.ListUsers(c.Request.Context(), operator, q, c.Query("keyword"))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, items)
}

// CheckRepeat 处理 GET /api/user/check-repeat。
func (h *Handler) CheckRepeat(c *gin.Context) {
	var req dto.CheckRepeatReq
	if !bindQuery(c, &req) {
		return
	}
	taken, err := h.svc.CheckRepeat(c.Request.Context(), req.Keyword)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, gin.H{"repeated": taken})
}

// Update 处理 PUT /api/user/:id，更新资料。
func (h *Handler) Update(c *gin.Context) {
	var req dto.UserUpdateReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	operator := model.UserInfo{BaseModel: model.BaseModel{ID: p.UserID}, Type: p.Type}
	targetID, _ := strconv.ParseUint(c.Query("id"), 10, 64)
	if targetID == 0 {
		targetID = p.UserID
	}
	u, err := h.svc.UpdateUser(c.Request.Context(), operator, targetID, req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, u)
}

// UpdatePwd 处理 PUT /api/user/password。
func (h *Handler) UpdatePwd(c *gin.Context) {
	var req dto.UpdatePwdReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	if err := h.svc.UpdatePwd(c.Request.Context(), p.UserID, req); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, nil)
}
