package handler

import (
	"io"
	"strconv"

	"apidoc/internal/dto"
	"apidoc/internal/pkg/errcode"
	"apidoc/internal/pkg/page"
	"apidoc/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// ---- 分组 ----

// CreateGroup 处理 POST /api/group。
func (h *Handler) CreateGroup(c *gin.Context) {
	var req dto.GroupCreateReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.CreateGroup(c.Request.Context(), p.UserID, p.Type, req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// ListGroups 处理 GET /api/group?project_id=&type=。
// ListDocGroups 文档分组树 GET /api/doc/groups?project_id=（B19）。
func (h *Handler) ListDocGroups(c *gin.Context) {
	p := principal(c)
	res, ungrouped, err := h.svc.ListDocGroups(c.Request.Context(), p.UserID, p.Type, idQuery(c, "project_id"))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, gin.H{"groups": res, "ungrouped": ungrouped})
}

func (h *Handler) ListGroups(c *gin.Context) {
	typ := int8(-1)
	if t, err := strconv.Atoi(c.Query("type")); err == nil {
		typ = int8(t)
	}
	p := principal(c)
	res, err := h.svc.ListGroups(c.Request.Context(), p.UserID, p.Type, idQuery(c, "project_id"), typ)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// UpdateGroup 处理 PUT /api/group。
func (h *Handler) UpdateGroup(c *gin.Context) {
	var req dto.GroupUpdateReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.UpdateGroup(c.Request.Context(), p.UserID, p.Type, req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// DeleteGroup 处理 DELETE /api/group/:id。
func (h *Handler) DeleteGroup(c *gin.Context) {
	p := principal(c)
	if err := h.svc.DeleteGroup(c.Request.Context(), p.UserID, p.Type, idParam(c)); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, nil)
}

// ---- 接口 ----

// CreateApi 处理 POST /api/api。
func (h *Handler) CreateApi(c *gin.Context) {
	var req dto.ApiCreateReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.CreateApi(c.Request.Context(), p.UserID, p.Type, req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// UpdateApi 处理 PUT /api/api。
func (h *Handler) UpdateApi(c *gin.Context) {
	var req dto.ApiUpdateReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.UpdateApi(c.Request.Context(), p.UserID, p.Type, req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// ListApis 处理 GET /api/api。
func (h *Handler) ListApis(c *gin.Context) {
	var q dto.ApiListQuery
	if !bindQuery(c, &q) {
		return
	}
	p := principal(c)
	res, err := h.svc.ListApis(c.Request.Context(), p.UserID, p.Type, q)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// GetApi 处理 GET /api/api/:id。
func (h *Handler) GetApi(c *gin.Context) {
	p := principal(c)
	res, err := h.svc.GetApi(c.Request.Context(), p.UserID, p.Type, idParam(c))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// DeleteApi 处理 DELETE /api/api/:id。
func (h *Handler) DeleteApi(c *gin.Context) {
	p := principal(c)
	if err := h.svc.DeleteApi(c.Request.Context(), p.UserID, p.Type, idParam(c)); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, nil)
}

// RestoreApi 处理 POST /api/api/restore（回收站单条恢复）。
func (h *Handler) RestoreApi(c *gin.Context) {
	var req dto.IDReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.RestoreApis(c.Request.Context(), p.UserID, p.Type, []uint64{req.ID})
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// RestoreApis 处理 POST /api/api/restore-batch（回收站批量恢复）。
func (h *Handler) RestoreApis(c *gin.Context) {
	var req dto.IDsReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.RestoreApis(c.Request.Context(), p.UserID, p.Type, req.IDs)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// PurgeApis 处理 POST /api/api/purge-batch（彻底删除，不可恢复）。
func (h *Handler) PurgeApis(c *gin.Context) {
	var req dto.IDsReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.PurgeApis(c.Request.Context(), p.UserID, p.Type, req.IDs)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// ---- 接口响应示例（多示例） ----

// SaveApiSample 处理 POST /api/api/sample（新增或更新）。
func (h *Handler) SaveApiSample(c *gin.Context) {
	var req dto.ApiSampleSaveReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.SaveApiSample(c.Request.Context(), p.UserID, p.Type, req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// ListApiSamples 处理 GET /api/api/samples。
func (h *Handler) ListApiSamples(c *gin.Context) {
	p := principal(c)
	var q dto.ApiSampleListQuery
	if !bindQuery(c, &q) {
		return
	}
	res, err := h.svc.ListApiSamples(c.Request.Context(), p.UserID, p.Type, q.ApiID)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// DeleteApiSample 处理 DELETE /api/api/sample。
func (h *Handler) DeleteApiSample(c *gin.Context) {
	var req dto.ApiSampleDeleteReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	if err := h.svc.DeleteApiSample(c.Request.Context(), p.UserID, p.Type, req.ID); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, nil)
}

// ImportOpenAPI 处理 POST /api/api/import，body 为标准 OpenAPI(JSON)。
func (h *Handler) ImportOpenAPI(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil || len(body) == 0 {
		response.Fail(c, errcode.CodeParamError, "请求体为空")
		return
	}
	p := principal(c)
	res, err := h.svc.ImportOpenAPI(c.Request.Context(), p.UserID, p.Type, idQuery(c, "project_id"), idQuery(c, "group_id"), c.Query("strategy"), body)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// ImportBruno 处理 POST /api/api/import-bruno，body 为 Bruno 集合目录内的文本文件（相对路径 + 内容）。
func (h *Handler) ImportBruno(c *gin.Context) {
	var req dto.BrunoImportReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.ImportBruno(c.Request.Context(), p.UserID, p.Type, idQuery(c, "project_id"), c.Query("strategy"), req.Files)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// ExportOpenAPI 处理 GET /api/api/export?project_id=&group_id=，导出标准 OpenAPI(JSON)。
func (h *Handler) ExportOpenAPI(c *gin.Context) {
	p := principal(c)
	doc, err := h.svc.OpenAPIExport(c.Request.Context(), p.UserID, p.Type, idQuery(c, "project_id"), idQuery(c, "group_id"))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, doc)
}

// ExportCurl 处理 GET /api/api/curl?id=&env_id=&resolve=，返回单接口的 cURL 命令。
// resolve=1 时用当前环境变量值替换 {{占位符}}（含敏感值，服务端渲染并写审计日志）。
func (h *Handler) ExportCurl(c *gin.Context) {
	p := principal(c)
	cmd, err := h.svc.ExportCurl(c.Request.Context(), p.UserID, p.Type, idParam(c), idQuery(c, "env_id"), c.Query("resolve") == "1")
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, gin.H{"curl": cmd})
}

// CodeSnippet 处理 GET /api/code?api_id=&env_id=&lang=&resolve=，返回多语言请求示例。
func (h *Handler) CodeSnippet(c *gin.Context) {
	p := principal(c)
	apiID := idOrFallback(c, "api_id")
	envID := idQuery(c, "env_id")
	resolve := c.Query("resolve") == "1"
	lang := c.Query("lang")
	code, err := h.svc.CodeSnippet(c.Request.Context(), p.UserID, p.Type, apiID, envID, lang, resolve)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, gin.H{"lang": lang, "code": code, "resolved": resolve})
}

// ---- 拖拽排序 ----

// SortNodes 处理 POST /api/sort，批量保存拖拽后的顺序。
func (h *Handler) SortNodes(c *gin.Context) {
	var req dto.SortReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	if err := h.svc.SortNodes(c.Request.Context(), p.UserID, p.Type, req); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, nil)
}

// ---- 文档 ----

// CreateDoc 处理 POST /api/doc。
func (h *Handler) CreateDoc(c *gin.Context) {
	var req dto.DocCreateReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.CreateDoc(c.Request.Context(), p.UserID, p.Type, req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// UpdateDoc 处理 PUT /api/doc。
func (h *Handler) UpdateDoc(c *gin.Context) {
	var req dto.DocUpdateReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.UpdateDoc(c.Request.Context(), p.UserID, p.Type, req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// ListDocs 处理 GET /api/doc。deleted=1 查回收站。
func (h *Handler) ListDocs(c *gin.Context) {
	var q page.Query
	_ = c.ShouldBindQuery(&q)
	deleted := int8(0)
	if c.Query("deleted") == "1" {
		deleted = 1
	}
	p := principal(c)
	res, err := h.svc.ListDocs(c.Request.Context(), p.UserID, p.Type, idQuery(c, "project_id"), idQuery(c, "group_id"), c.Query("keyword"), deleted, q)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// GetDoc 处理 GET /api/doc/:id。
func (h *Handler) GetDoc(c *gin.Context) {
	p := principal(c)
	res, err := h.svc.GetDoc(c.Request.Context(), p.UserID, p.Type, idParam(c))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// DeleteDoc 处理 DELETE /api/doc/:id。
func (h *Handler) DeleteDoc(c *gin.Context) {
	p := principal(c)
	if err := h.svc.DeleteDoc(c.Request.Context(), p.UserID, p.Type, idParam(c)); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, nil)
}

// RestoreDoc 处理 POST /api/doc/restore（回收站单条恢复）。
func (h *Handler) RestoreDoc(c *gin.Context) {
	var req dto.IDReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.RestoreDocs(c.Request.Context(), p.UserID, p.Type, []uint64{req.ID})
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// RestoreDocs 处理 POST /api/doc/restore-batch（回收站批量恢复）。
func (h *Handler) RestoreDocs(c *gin.Context) {
	var req dto.IDsReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.RestoreDocs(c.Request.Context(), p.UserID, p.Type, req.IDs)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// PurgeDocs 处理 POST /api/doc/purge-batch（彻底删除，不可恢复）。
func (h *Handler) PurgeDocs(c *gin.Context) {
	var req dto.IDsReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.PurgeDocs(c.Request.Context(), p.UserID, p.Type, req.IDs)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// LikeDoc 处理 POST /api/doc/:id/like。
func (h *Handler) LikeDoc(c *gin.Context) {
	p := principal(c)
	if err := h.svc.LikeDoc(c.Request.Context(), p.UserID, p.Type, idParam(c)); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, nil)
}

// ---- 字段映射 ----

// CreateFieldMapping 处理 POST /api/field-mapping。
func (h *Handler) CreateFieldMapping(c *gin.Context) {
	var req dto.FieldMappingReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.CreateFieldMapping(c.Request.Context(), p.UserID, p.Type, req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// ListFieldMappings 处理 GET /api/field-mapping?project_id=。
func (h *Handler) ListFieldMappings(c *gin.Context) {
	p := principal(c)
	res, err := h.svc.ListFieldMappings(c.Request.Context(), p.UserID, p.Type, idQuery(c, "project_id"))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// UpdateFieldMapping 处理 PUT /api/field-mapping。
func (h *Handler) UpdateFieldMapping(c *gin.Context) {
	var req dto.FieldMappingReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.UpdateFieldMapping(c.Request.Context(), p.UserID, p.Type, req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// DeleteFieldMapping 处理 DELETE /api/field-mapping/:id。
func (h *Handler) DeleteFieldMapping(c *gin.Context) {
	p := principal(c)
	if err := h.svc.DeleteFieldMapping(c.Request.Context(), p.UserID, p.Type, idParam(c)); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, nil)
}
