// 变量与环境接口：项目级环境 CRUD/引用预检 + 三级变量 CRUD + 解析表 + 列表徽标批量统计。
package handler

import (
	"strconv"
	"strings"

	"apidoc/internal/dto"
	"apidoc/internal/pkg/errcode"
	"apidoc/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// CreateVariableEnv 处理 POST /api/variable-env。
func (h *Handler) CreateVariableEnv(c *gin.Context) {
	var req dto.VariableEnvReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.CreateVariableEnv(c.Request.Context(), p.UserID, p.Type, req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// ListVariableEnvs 处理 GET /api/variable-env?project_id=。
func (h *Handler) ListVariableEnvs(c *gin.Context) {
	res, err := h.svc.ListVariableEnvs(c.Request.Context(), idQuery(c, "project_id"))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// UpdateVariableEnv 处理 PUT /api/variable-env。
func (h *Handler) UpdateVariableEnv(c *gin.Context) {
	var req dto.VariableEnvReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.UpdateVariableEnv(c.Request.Context(), p.UserID, p.Type, req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// SetDefaultVariableEnv 处理 PUT /api/variable-env/default?id=。
func (h *Handler) SetDefaultVariableEnv(c *gin.Context) {
	p := principal(c)
	if err := h.svc.SetDefaultVariableEnv(c.Request.Context(), p.UserID, p.Type, idParam(c)); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, nil)
}

// VariableEnvRefs 处理 GET /api/variable-env/references?id=（删除前预检）。
func (h *Handler) VariableEnvRefs(c *gin.Context) {
	res, err := h.svc.VariableEnvRefs(c.Request.Context(), idParam(c))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// DeleteVariableEnv 处理 DELETE /api/variable-env?id=。
func (h *Handler) DeleteVariableEnv(c *gin.Context) {
	p := principal(c)
	if err := h.svc.DeleteVariableEnv(c.Request.Context(), p.UserID, p.Type, idParam(c)); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, nil)
}

// CreateVariable 处理 POST /api/variable。
func (h *Handler) CreateVariable(c *gin.Context) {
	var req dto.VariableReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.CreateVariable(c.Request.Context(), p.UserID, p.Type, req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// ListVariables 处理 GET /api/variable?project_id=&scope=&api_id=&env_id=&keyword=&with_value=。
// with_value=1 时对具备写权限者回明文（供编辑回显，敏感值会记操作日志）。
func (h *Handler) ListVariables(c *gin.Context) {
	var q dto.VariableListQuery
	if !bindQuery(c, &q) {
		return
	}
	p := principal(c)
	res, err := h.svc.ListVariables(c.Request.Context(), p.UserID, p.Type, q, c.Query("with_value") == "1")
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// UpdateVariable 处理 PUT /api/variable。
func (h *Handler) UpdateVariable(c *gin.Context) {
	var req dto.VariableReq
	if !bindJSON(c, &req) {
		return
	}
	p := principal(c)
	res, err := h.svc.UpdateVariable(c.Request.Context(), p.UserID, p.Type, req)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// DeleteVariable 处理 DELETE /api/variable?id=。
func (h *Handler) DeleteVariable(c *gin.Context) {
	p := principal(c)
	if err := h.svc.DeleteVariable(c.Request.Context(), p.UserID, p.Type, idParam(c)); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, nil)
}

// ResolveVariables 处理 GET /api/variable/resolve?project_id=&api_id=&env_id=。
func (h *Handler) ResolveVariables(c *gin.Context) {
	res, err := h.svc.ResolveVariables(c.Request.Context(),
		idQuery(c, "project_id"), idQuery(c, "api_id"), idQuery(c, "env_id"))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// SummarizeVariables 处理 GET /api/variable/summary?project_id=&env_id=&api_ids=1,2,3。
func (h *Handler) SummarizeVariables(c *gin.Context) {
	var q dto.VariableSummaryQuery
	if !bindQuery(c, &q) {
		return
	}
	ids, err := parseIDList(q.ApiIDs)
	if err != nil || len(ids) == 0 {
		response.Fail(c, errcode.CodeParamError, "api_ids 需为逗号分隔的接口 id")
		return
	}
	res, err := h.svc.SummarizeVariables(c.Request.Context(), q.ProjectID, q.EnvID, ids)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, res)
}

// parseIDList 解析逗号分隔的 id 列表（忽略空白与 0，去重保序）。
func parseIDList(raw string) ([]uint64, error) {
	seen := map[uint64]struct{}{}
	out := make([]uint64, 0)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return nil, err
		}
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}
