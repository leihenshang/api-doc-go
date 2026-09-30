package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"apidoc/internal/dto"
	"apidoc/internal/pkg/errcode"
	"apidoc/internal/pkg/fileutil"
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

// ---- 文件上传 ----

// UploadImg 处理 POST /api/file/upload（multipart 字段 file）。
// 加固：大小上限 → 扩展名白名单 → 文件头嗅探（防改名绕过）→ 随机文件名（防枚举）。
func (h *Handler) UploadImg(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, errcode.CodeParamError, "缺少文件")
		return
	}
	if maxBytes := h.cfg.Upload.MaxMB * 1024 * 1024; maxBytes > 0 && fileHeader.Size > maxBytes {
		response.Fail(c, errcode.CodeParamError, fmt.Sprintf("文件过大，单文件上限 %d MB", h.cfg.Upload.MaxMB))
		return
	}
	ext := fileutil.NormalizeExt(fileHeader.Filename)
	if !fileutil.Allowed(ext, h.cfg.Upload.AllowExt) {
		response.Fail(c, errcode.CodeParamError, "不支持的文件类型，允许："+strings.Join(h.cfg.Upload.AllowExt, " "))
		return
	}
	// 嗅探文件头：内容类型必须与扩展名同族，防止把 HTML/脚本改名为图片上传。
	src, err := fileHeader.Open()
	if err != nil {
		fail(c, err)
		return
	}
	head := make([]byte, 512)
	n, _ := io.ReadFull(src, head)
	_ = src.Close()
	if !fileutil.SniffAllowed(head[:n], ext) {
		response.Fail(c, errcode.CodeParamError, "文件内容与扩展名不符，已拒绝上传")
		return
	}
	name, err := fileutil.RandomName(ext)
	if err != nil {
		fail(c, err)
		return
	}
	dst := filepath.Join(h.cfg.Upload.Dir, name)
	if err := c.SaveUploadedFile(fileHeader, dst); err != nil {
		fail(c, err)
		return
	}
	// S13 内容寻址：算 sha256 落库
	data, _ := os.ReadFile(dst)
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	f, err := h.svc.SaveFileWithHash(c.Request.Context(), fileHeader.Filename, "/uploads/"+name, hash, fileHeader.Header.Get("Content-Type"), fileHeader.Size)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": f.ID, "url": "/uploads/" + name, "hash": hash})
}

// GetFileByHash 处理 GET /api/file/:hash — 内容寻址取文件（S13 / R9）。
func (h *Handler) GetFileByHash(c *gin.Context) {
	hash := c.Param("hash")
	if hash == "" {
		response.Fail(c, errcode.CodeParamError, "hash 必填")
		return
	}
	f, err := h.svc.GetFileByHash(c.Request.Context(), hash)
	if err != nil {
		response.Fail(c, errcode.CodeFailed, "文件不存在")
		return
	}
	// 返回文件内容（离线映射用）
	local := strings.TrimPrefix(f.Path, "/uploads/")
	full := filepath.Join(h.cfg.Upload.Dir, local)
	if _, err := os.Stat(full); err != nil {
		response.Fail(c, errcode.CodeFailed, "文件丢失")
		return
	}
	ct := f.Mime
	if ct == "" {
		ct = "application/octet-stream"
	}
	c.Header("Content-Type", ct)
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.File(full)
}

// ServeUpload 处理 GET /uploads/*path：代替 engine.Static 的安全静态服务。
// 只服务白名单扩展名 → 按扩展名固定 Content-Type（绝不 text/html）→ nosniff + inline；
// 配置了 UPLOAD_SIGN_KEY 时校验 `?s=` 签名（默认关闭，保证分享页/文档内嵌图片可用）。
func (h *Handler) ServeUpload(c *gin.Context) {
	name := filepath.Base(strings.TrimPrefix(c.Param("path"), "/"))
	if name == "" || name == "." || name == ".." || name == "/" {
		c.Status(http.StatusNotFound)
		return
	}
	ext := fileutil.NormalizeExt(name)
	if !fileutil.Allowed(ext, h.cfg.Upload.AllowExt) {
		c.Status(http.StatusNotFound)
		return
	}
	if h.cfg.Upload.SignKey != "" && c.Query("s") != fileutil.Sign(name, h.cfg.Upload.SignKey) {
		c.Status(http.StatusForbidden)
		return
	}
	path := filepath.Join(h.cfg.Upload.Dir, name)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Type", fileutil.ContentTypeOf(ext))
	c.Header("Content-Disposition", "inline")
	c.Header("Cache-Control", "public, max-age=86400")
	c.File(path)
}
