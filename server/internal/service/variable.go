// 变量与环境：三级变量（请求级 API / 项目级 / 全局应用级）+ 项目级环境。
// 解析优先级：请求级 > 项目级(当前环境) > 项目级(通用 env_id=0) > 全局。
// 敏感值（is_secret=1）明文入库，仅对外脱敏：列表与解析只回掩码，
// 编辑回显需显式 with_value=1 且具备写权限（并写操作日志）。详见 doc/变量与环境功能规划.md。
package service

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/leihenshang/api-doc-go-share/varx"

	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
	"apidoc/internal/repository"
)

// 变量名规则（写入校验）：字母或下划线开头，仅含字母/数字/下划线，最长 64 位。
// 渲染侧的占位符语法与优先级由共享包 varx 统一，不在本层复制一份规则。
var varNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)

// varMaskedValue 敏感值对外掩码（取值与共享规则同源）。
const varMaskedValue = varx.MaskedValue

// 变量解析来源标记（与前端展示一致）。
const (
	varSourceRequest = "request"
	varSourceProject = "project"
	varSourceGlobal  = "global"
)

// maxSummaryApiIDs 单次批量徽标查询的接口数上限（超出由前端分批）。
const maxSummaryApiIDs = 200

// VarView 是变量对外视图：敏感值一律掩码（唯一出口，避免明文外泄）。
type VarView struct {
	ID          uint64    `json:"id"`
	Scope       int8      `json:"scope"`
	ProjectID   uint64    `json:"project_id"`
	ApiID       uint64    `json:"api_id"`
	EnvID       uint64    `json:"env_id"`
	Name        string    `json:"name"`
	Value       string    `json:"value"`
	Description string    `json:"description"`
	IsSecret    int8      `json:"is_secret"`
	HasValue    bool      `json:"has_value"`
	Enabled     int8      `json:"enabled"`
	Priority    int       `json:"priority"`
	CreateTime  time.Time `json:"create_time"`
	UpdateTime  time.Time `json:"update_time"`
}

// ResolveItem 解析结果项：变量视图 + 生效来源（request/project/global）。
type ResolveItem struct {
	VarView
	Source string `json:"source"`
}

// EnvBrief 环境摘要（引用检查返回）。
type EnvBrief struct {
	ID        uint64 `json:"id"`
	Name      string `json:"name"`
	IsDefault int8   `json:"is_default"`
}

// EnvRefs 环境引用汇总：删除环境前的预检结果。
type EnvRefs struct {
	Env       EnvBrief  `json:"env"`
	Variables []VarView `json:"variables"`
	Blocked   bool      `json:"blocked"`
}

// VarSummary 单个接口的变量概览（接口列表徽标）。
type VarSummary struct {
	Defined        []string `json:"defined"`
	Undefined      []string `json:"undefined"`
	RequestCount   int      `json:"request_count"`
	UndefinedCount int      `json:"undefined_count"`
}

// resolvedVar 解析中间结果：命中的变量与其优先级。
type resolvedVar struct {
	v    model.Variable
	rank int
}

// ================= 环境 =================

// CreateVariableEnv 新建环境；项目首个环境自动成为默认环境。
// logVarChange 变量/环境的变更流水（P0-7）：Web 写路径此前完全不写 change_log，
// 客户端增量拉取永远收不到这类变更。ext_uid 为空（非同步实体）时跳过。
func (s *Service) logVarChange(ctx context.Context, projectID uint64, entityType string, id uint64, extUID, op string, actor uint64) {
	if extUID == "" {
		return
	}
	// 流水与 last_change_id 同事务写入（B-12/B-15），失败不回滚已落库的变量。
	_, _ = s.appendChangeTx(ctx, nil, entityType, extUID, projectID, id, op, actor, "")
}

// CreateVariable 新建变量（请求级 / 项目级 / 全局）。
func (s *Service) CreateVariable(ctx context.Context, userID uint64, typ int8, req dto.VariableReq) (*VarView, error) {
	v, err := s.normalizeVariable(ctx, req, 0)
	if err != nil {
		return nil, err
	}
	if err := s.ensureVariableWritable(ctx, userID, typ, v, model.PermCreate); err != nil {
		return nil, err
	}
	if err := s.repo.CreateVariable(ctx, v); err != nil {
		return nil, err
	}
	s.recordOp(ctx, v.ProjectID, userID, v.ID, model.LogObjectVariable, "create", "变量:"+v.Name)
	s.logVarChange(ctx, v.ProjectID, model.SyncTypeVariable, v.ID, v.ExtUID, "upsert", userID)
	view := maskVariable(*v)
	return &view, nil
}

// ListVariables 列出变量；withValue=true 且具备写权限时回明文（供编辑回显，敏感值也回明文并记日志）。
func (s *Service) ListVariables(ctx context.Context, userID uint64, typ int8, q dto.VariableListQuery, withValue bool) ([]VarView, error) {
	// P0-1：变量列表含名称/值/所属接口，非项目成员不得读取（此前只对「明文」做了权限判断）
	if err := s.ensureProjectPermission(ctx, userID, typ, q.ProjectID, model.PermQuery); err != nil {
		return nil, err
	}
	items, err := s.repo.ListVariables(ctx, repository.VariableFilter{
		ProjectID: q.ProjectID, Scope: q.Scope, ApiID: q.ApiID, EnvID: q.EnvID, Keyword: q.Keyword,
	})
	if err != nil {
		return nil, err
	}
	plain := withValue && s.canRevealValues(ctx, userID, typ, q.ProjectID)
	views := make([]VarView, 0, len(items))
	revealed := make([]string, 0)
	for _, v := range items {
		// 全局变量的明文只对管理员回显。
		rowPlain := plain && (v.Scope != model.VarScopeGlobal || isAdmin(typ))
		if rowPlain {
			if v.IsSecret == model.VarSecret {
				revealed = append(revealed, v.Name)
			}
			views = append(views, plainVariable(v))
			continue
		}
		views = append(views, maskVariable(v))
	}
	if len(revealed) > 0 {
		s.recordOp(ctx, q.ProjectID, userID, 0, model.LogObjectVariable, "query", "查看敏感变量明文："+strings.Join(revealed, ", "))
	}
	return views, nil
}

// UpdateVariable 修改变量（作用域不可变更）。
func (s *Service) UpdateVariable(ctx context.Context, userID uint64, typ int8, req dto.VariableReq) (*VarView, error) {
	old, err := s.repo.GetVariable(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureVariableWritable(ctx, userID, typ, old, model.PermUpdate); err != nil {
		return nil, err
	}
	// 乐观锁：客户端载入后若他人已保存，则拒绝覆盖（未传 expected_update_time 时不做校验）。
	if err := ensureNotStale(req.ExpectedUpdateTime, old.UpdateTime, "变量"); err != nil {
		return nil, err
	}
	if req.Scope != old.Scope {
		return nil, newBiz(errcode.CodeParamError, "变量作用域不可修改，请新建变量")
	}
	// 未传的归属字段回落到原值，避免前端只改值时报错。
	if req.ProjectID == 0 {
		req.ProjectID = old.ProjectID
	}
	if req.ApiID == 0 {
		req.ApiID = old.ApiID
	}
	next, err := s.normalizeVariable(ctx, req, old.ID)
	if err != nil {
		return nil, err
	}
	next.ID = old.ID
	next.BaseModel = old.BaseModel
	if req.Value == nil {
		next.Value = old.Value
	}
	if req.Enabled == nil {
		next.Enabled = old.Enabled
	}
	if err := s.repo.UpdateVariable(ctx, next); err != nil {
		return nil, err
	}
	s.recordOp(ctx, next.ProjectID, userID, next.ID, model.LogObjectVariable, "update", "变量:"+next.Name)
	s.logVarChange(ctx, next.ProjectID, model.SyncTypeVariable, next.ID, next.ExtUID, "upsert", userID)
	view := maskVariable(*next)
	return &view, nil
}

// DeleteVariable 软删除变量。
func (s *Service) DeleteVariable(ctx context.Context, userID uint64, typ int8, id uint64) error {
	v, err := s.repo.GetVariable(ctx, id)
	if err != nil {
		return err
	}
	// 请求变量随接口内容维护，用 update 位；项目变量用 delete 位；全局变量仅管理员。
	need := int8(model.PermDelete)
	if v.Scope == model.VarScopeRequest {
		need = model.PermUpdate
	}
	if err := s.ensureVariableWritable(ctx, userID, typ, v, need); err != nil {
		return err
	}
	if err := s.repo.DeleteVariable(ctx, id); err != nil {
		return err
	}
	s.recordOp(ctx, v.ProjectID, userID, v.ID, model.LogObjectVariable, "delete", "变量:"+v.Name)
	s.logVarChange(ctx, v.ProjectID, model.SyncTypeVariable, v.ID, v.ExtUID, "delete", userID)
	return nil
}

// ================= 内部工具 =================

// normalizeVariable 校验并组装变量实体：作用域归属、名称规则、同层同环境唯一。
func (s *Service) normalizeVariable(ctx context.Context, req dto.VariableReq, excludeID uint64) (*model.Variable, error) {
	if req.Scope != model.VarScopeRequest && req.Scope != model.VarScopeProject && req.Scope != model.VarScopeGlobal {
		return nil, newBiz(errcode.CodeParamError, "变量作用域不合法")
	}
	name := strings.TrimSpace(req.Name)
	if !varNamePattern.MatchString(name) {
		return nil, newBiz(errcode.CodeParamError, "变量名需以字母或下划线开头，仅含字母、数字与下划线，最长 64 位")
	}
	v := &model.Variable{
		Scope:       req.Scope,
		Name:        name,
		Value:       derefString(req.Value),
		Description: req.Description,
		IsSecret:    req.IsSecret,
		Priority:    req.Priority,
		Enabled:     model.VarEnabled,
	}
	// P0-7 深层原因：Web 新建变量必须有 ext_uid，否则 logVarChange 直接跳过（客户端收不到）
	v.ExtUID = ensureExtUID("")
	if req.Enabled != nil {
		v.Enabled = *req.Enabled
	}
	switch req.Scope {
	case model.VarScopeRequest:
		if req.ProjectID == 0 || req.ApiID == 0 {
			return nil, newBiz(errcode.CodeParamError, "请求变量必须指定所属接口")
		}
		a, err := s.repo.GetApi(ctx, req.ApiID)
		if err != nil {
			return nil, err
		}
		if a.ProjectID != req.ProjectID {
			return nil, newBiz(errcode.CodeParamError, "接口不属于该项目")
		}
		v.ProjectID, v.ApiID = req.ProjectID, req.ApiID
	case model.VarScopeProject:
		if req.ProjectID == 0 {
			return nil, newBiz(errcode.CodeParamError, "项目变量必须指定所属项目")
		}
		v.ProjectID = req.ProjectID
		if req.EnvID > 0 {
			env, err := s.repo.GetVariableEnv(ctx, req.EnvID)
			if err != nil {
				return nil, err
			}
			if env.ProjectID != req.ProjectID {
				return nil, newBiz(errcode.CodeParamError, "环境不属于该项目")
			}
			v.EnvID = req.EnvID
		}
	case model.VarScopeGlobal:
		// 全局变量跨项目共享：不绑定项目/接口/环境。
	}
	taken, err := s.repo.VariableTaken(ctx, v.Scope, v.ProjectID, v.ApiID, v.EnvID, v.Name, excludeID)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, newBiz(errcode.CodeFailed, "同名变量「%s」已存在（%s）", v.Name, scopeLabel(v))
	}
	return v, nil
}

// ensureVariableWritable 按作用域校验写权限：请求级/项目级走项目权限位，全局变量仅管理员。
func (s *Service) ensureVariableWritable(ctx context.Context, userID uint64, typ int8, v *model.Variable, need int8) error {
	if v.Scope == model.VarScopeGlobal {
		return s.ensureAdmin(typ)
	}
	return s.ensureProjectPermission(ctx, userID, typ, v.ProjectID, need)
}

// canRevealValues 判断是否允许读取敏感值明文：管理员放行，项目内需负责人或 update 权限。
func (s *Service) canRevealValues(ctx context.Context, userID uint64, typ int8, projectID uint64) bool {
	if isAdmin(typ) {
		return true
	}
	if projectID == 0 {
		return false
	}
	m, err := s.repo.Member(ctx, userID, projectID)
	if err != nil {
		return false
	}
	return m.IsLeader == 1 || model.Has(m.Permission, model.PermUpdate)
}
