package handler

import (
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"apidoc/internal/dto"
	"apidoc/internal/pkg/errcode"
	"apidoc/internal/pkg/page"
	"apidoc/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// ---- Bug ----

// CreateBug 处理 POST /api/bug。
func (h *Handler) CreateBug(c *gin.Context) {
	var req dto.BugCreateReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.CreateBug(c.Request.Context(), p.UserID, p.Type, req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// UpdateBug 处理 PUT /api/bug。
func (h *Handler) UpdateBug(c *gin.Context) {
	var req dto.BugUpdateReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.UpdateBug(c.Request.Context(), p.UserID, p.Type, req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// AssignBug 处理 POST /api/bug/assign。
func (h *Handler) AssignBug(c *gin.Context) {
	var req dto.BugAssignReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	if err := h.svc.AssignBug(c.Request.Context(), p.UserID, p.Type, req); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, nil)
}

// ListBugs 处理 GET /api/bug。
func (h *Handler) ListBugs(c *gin.Context) {
	var q page.Query
	_ = c.ShouldBindQuery(&q)
	status := int8(0)
	if v, err := strconv.Atoi(c.Query("status")); err == nil {
		status = int8(v)
	}
	res, err := h.svc.ListBugs(c.Request.Context(), idQuery(c, "project_id"), status, q)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// GetBug 处理 GET /api/bug/:id。
func (h *Handler) GetBug(c *gin.Context) {
	res, err := h.svc.GetBug(c.Request.Context(), idParam(c))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// ListBugAssign 处理 GET /api/bug/assign。
func (h *Handler) ListBugAssign(c *gin.Context) {
	res, err := h.svc.ListBugAssign(c.Request.Context(), idOrFallback(c, "bug_id"))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// DeleteBug 处理 DELETE /api/bug/:id。
func (h *Handler) DeleteBug(c *gin.Context) {
	p := principal(c)
	if err := h.svc.DeleteBug(c.Request.Context(), p.UserID, p.Type, idParam(c)); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, nil)
}

// ---- 公共属性 ----

// ListProperties 处理 GET /api/property。
func (h *Handler) ListProperties(c *gin.Context) {
	res, err := h.svc.ListProperties(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// SaveProperty 处理 POST /api/property（id 为 0 新建，否则更新）。
func (h *Handler) SaveProperty(c *gin.Context) {
	var req dto.PropertyReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.SaveProperty(c.Request.Context(), p.Type, req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// DeleteProperty 处理 DELETE /api/property/:id。
func (h *Handler) DeleteProperty(c *gin.Context) {
	p := principal(c)
	if err := h.svc.DeleteProperty(c.Request.Context(), p.Type, idParam(c)); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, nil)
}

// ---- 消息 / 通知 ----

// SendMessage 处理 POST /api/message/send。
func (h *Handler) SendMessage(c *gin.Context) {
	var req dto.MessageSendReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.svc.SendMessage(c.Request.Context(), req); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, nil)
}

// ListNotifications 处理 GET /api/notification。
func (h *Handler) ListNotifications(c *gin.Context) {
	var q page.Query
	_ = c.ShouldBindQuery(&q)
	p := principal(c)
	res, err := h.svc.ListNotifications(c.Request.Context(), p.UserID, q)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// MarkNotificationRead 处理 POST /api/notification/read。
func (h *Handler) MarkNotificationRead(c *gin.Context) {
	var req dto.IDReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	if err := h.svc.MarkNotificationRead(c.Request.Context(), p.UserID, req.ID); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, nil)
}

// ---- 操作日志 ----

// ListLogs 处理 GET /api/operation-log。
func (h *Handler) ListLogs(c *gin.Context) {
	var q page.Query
	_ = c.ShouldBindQuery(&q)
	typ := int8(0)
	if v, err := strconv.Atoi(c.Query("type")); err == nil {
		typ = int8(v)
	}
	res, err := h.svc.ListLogs(c.Request.Context(), idQuery(c, "project_id"), idQuery(c, "object_id"), typ, q)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// ---- 外部数据库连接 ----

// CreateConn 处理 POST /api/database。
func (h *Handler) CreateConn(c *gin.Context) {
	var req dto.DatabaseReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.CreateConn(c.Request.Context(), p.UserID, p.Type, req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// ListConns 处理 GET /api/database?project_id=。
func (h *Handler) ListConns(c *gin.Context) {
	res, err := h.svc.ListConns(c.Request.Context(), idQuery(c, "project_id"))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// UpdateConn 处理 PUT /api/database。
func (h *Handler) UpdateConn(c *gin.Context) {
	var req dto.DatabaseReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.UpdateConn(c.Request.Context(), p.UserID, p.Type, req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// DeleteConn 处理 DELETE /api/database/:id。
func (h *Handler) DeleteConn(c *gin.Context) {
	p := principal(c)
	if err := h.svc.DeleteConn(c.Request.Context(), p.UserID, p.Type, idParam(c)); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, nil)
}

// ConnTables 处理 GET /api/database/tables?conn_id=，读取外部库表清单。
func (h *Handler) ConnTables(c *gin.Context) {
	p := principal(c)
	res, err := h.svc.ConnTables(c.Request.Context(), p.UserID, p.Type, idQuery(c, "conn_id"))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// ConnColumns 处理 GET /api/database/columns?conn_id=&table=，读取表列结构。
func (h *Handler) ConnColumns(c *gin.Context) {
	p := principal(c)
	res, err := h.svc.ConnColumns(c.Request.Context(), p.UserID, p.Type, idQuery(c, "conn_id"), c.Query("table"))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// ---- 文件上传 ----

// UploadImg 处理 POST /api/file/upload（multipart 字段 file）。
func (h *Handler) UploadImg(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, errcode.CodeParamError, "缺少文件")
		return
	}
	ext := filepath.Ext(fileHeader.Filename)
	name := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	dst := filepath.Join(h.cfg.Upload.Dir, name)
	if err := c.SaveUploadedFile(fileHeader, dst); err != nil {
		fail(c, err)
		return
	}
	f, err := h.svc.SaveFile(c.Request.Context(), fileHeader.Filename, "/uploads/"+name)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": f.ID, "url": "/uploads/" + name})
}
