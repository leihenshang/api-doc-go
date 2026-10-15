// 变量与环境：三级变量（请求级 API / 项目级 / 全局应用级）+ 项目级环境。
// 解析优先级：请求级 > 项目级(当前环境) > 项目级(通用 env_id=0) > 全局。
// 敏感值（is_secret=1）明文入库，仅对外脱敏：列表与解析只回掩码，
// 编辑回显需显式 with_value=1 且具备写权限（并写操作日志）。详见 doc/变量与环境功能规划.md。
package service

import (
	"context"
	"regexp"
	"sort"
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
func (s *Service) CreateVariableEnv(ctx context.Context, userID uint64, typ int8, req dto.VariableEnvReq) (*model.VariableEnv, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, newBiz(errcode.CodeParamError, "环境名称不能为空")
	}
	if err := s.ensureProjectPermission(ctx, userID, typ, req.ProjectID, model.PermCreate); err != nil {
		return nil, err
	}
	taken, err := s.repo.VariableEnvTaken(ctx, req.ProjectID, name, 0)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, newBiz(errcode.CodeFailed, "环境「%s」已存在", name)
	}
	n, err := s.repo.CountVariableEnvs(ctx, req.ProjectID)
	if err != nil {
		return nil, err
	}
	env := &model.VariableEnv{
		ProjectID:   req.ProjectID,
		Name:        name,
		Description: req.Description,
		Color:       strings.TrimSpace(req.Color),
		Priority:    req.Priority,
	}
	if n == 0 {
		env.IsDefault = model.VarEnvDefault
	}
	if err := s.repo.CreateVariableEnv(ctx, env); err != nil {
		return nil, err
	}
	s.recordOp(ctx, env.ProjectID, userID, env.ID, model.LogObjectVariableEnv, "create", "环境:"+env.Name)
	return env, nil
}

// ListVariableEnvs 列出项目环境（默认环境优先）。
func (s *Service) ListVariableEnvs(ctx context.Context, projectID uint64) ([]model.VariableEnv, error) {
	if projectID == 0 {
		return nil, newBiz(errcode.CodeParamError, "缺少 project_id")
	}
	return s.repo.ListVariableEnvs(ctx, projectID)
}

// UpdateVariableEnv 更新环境名称/描述/颜色/排序。
func (s *Service) UpdateVariableEnv(ctx context.Context, userID uint64, typ int8, req dto.VariableEnvReq) (*model.VariableEnv, error) {
	env, err := s.repo.GetVariableEnv(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureProjectPermission(ctx, userID, typ, env.ProjectID, model.PermUpdate); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, newBiz(errcode.CodeParamError, "环境名称不能为空")
	}
	taken, err := s.repo.VariableEnvTaken(ctx, env.ProjectID, name, env.ID)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, newBiz(errcode.CodeFailed, "环境「%s」已存在", name)
	}
	env.Name = name
	env.Description = req.Description
	env.Color = strings.TrimSpace(req.Color)
	env.Priority = req.Priority
	if err := s.repo.UpdateVariableEnv(ctx, env); err != nil {
		return nil, err
	}
	s.recordOp(ctx, env.ProjectID, userID, env.ID, model.LogObjectVariableEnv, "update", "环境:"+env.Name)
	return env, nil
}

// SetDefaultVariableEnv 把某环境设为项目默认环境（同项目互斥）。
func (s *Service) SetDefaultVariableEnv(ctx context.Context, userID uint64, typ int8, id uint64) error {
	env, err := s.repo.GetVariableEnv(ctx, id)
	if err != nil {
		return err
	}
	if err := s.ensureProjectPermission(ctx, userID, typ, env.ProjectID, model.PermUpdate); err != nil {
		return err
	}
	if err := s.repo.ClearVariableEnvDefault(ctx, env.ProjectID, env.ID); err != nil {
		return err
	}
	env.IsDefault = model.VarEnvDefault
	if err := s.repo.UpdateVariableEnv(ctx, env); err != nil {
		return err
	}
	s.recordOp(ctx, env.ProjectID, userID, env.ID, model.LogObjectVariableEnv, "update", "默认环境:"+env.Name)
	return nil
}

// VariableEnvRefs 返回引用某环境的资源明细（删除前预检）。
func (s *Service) VariableEnvRefs(ctx context.Context, id uint64) (*EnvRefs, error) {
	env, err := s.repo.GetVariableEnv(ctx, id)
	if err != nil {
		return nil, err
	}
	vars, err := s.repo.VariablesByEnv(ctx, id)
	if err != nil {
		return nil, err
	}
	return &EnvRefs{
		Env:       EnvBrief{ID: env.ID, Name: env.Name, IsDefault: env.IsDefault},
		Variables: maskVariables(vars),
		Blocked:   env.IsDefault == model.VarEnvDefault || len(vars) > 0,
	}, nil
}

// DeleteVariableEnv 删除环境：默认环境与「仍被变量引用」的环境一律拒绝，并给出明细。
func (s *Service) DeleteVariableEnv(ctx context.Context, userID uint64, typ int8, id uint64) error {
	env, err := s.repo.GetVariableEnv(ctx, id)
	if err != nil {
		return err
	}
	if err := s.ensureProjectPermission(ctx, userID, typ, env.ProjectID, model.PermDelete); err != nil {
		return err
	}
	if env.IsDefault == model.VarEnvDefault {
		// 仅当项目还有其它环境时才要求先改默认；唯一的默认环境允许删除（项目回到「无环境」状态）。
		n, err := s.repo.CountVariableEnvs(ctx, env.ProjectID)
		if err != nil {
			return err
		}
		if n > 1 {
			return newBiz(errcode.CodeFailed, "「%s」是项目默认环境，请先把其它环境设为默认", env.Name)
		}
	}
	vars, err := s.repo.VariablesByEnv(ctx, id)
	if err != nil {
		return err
	}
	if len(vars) > 0 {
		return newBiz(errcode.CodeFailed, "环境「%s」被 %d 个变量引用，无法删除：%s；请先删除或改绑这些变量",
			env.Name, len(vars), joinNames(vars, 3))
	}
	if err := s.repo.DeleteVariableEnv(ctx, id); err != nil {
		return err
	}
	s.recordOp(ctx, env.ProjectID, userID, env.ID, model.LogObjectVariableEnv, "delete", "环境:"+env.Name)
	return nil
}

// ================= 变量 =================

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
	view := maskVariable(*v)
	return &view, nil
}

// ListVariables 列出变量；withValue=true 且具备写权限时回明文（供编辑回显，敏感值也回明文并记日志）。
func (s *Service) ListVariables(ctx context.Context, userID uint64, typ int8, q dto.VariableListQuery, withValue bool) ([]VarView, error) {
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
	return nil
}

// ResolveVariables 解析接口可见变量表：同名只保留最高优先级的一条，敏感值掩码。
func (s *Service) ResolveVariables(ctx context.Context, projectID, apiID, envID uint64) ([]ResolveItem, error) {
	if projectID == 0 {
		return nil, newBiz(errcode.CodeParamError, "缺少 project_id")
	}
	vars, err := s.repo.VariablesForResolve(ctx, projectID, apiID)
	if err != nil {
		return nil, err
	}
	best := map[string]resolvedVar{}
	for _, v := range vars {
		rank, ok := varRank(v, envID)
		if !ok {
			continue
		}
		if cur, exists := best[v.Name]; !exists || rank < cur.rank {
			best[v.Name] = resolvedVar{v: v, rank: rank}
		}
	}
	names := make([]string, 0, len(best))
	for name := range best {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]ResolveItem, 0, len(names))
	for _, name := range names {
		hit := best[name]
		out = append(out, ResolveItem{VarView: maskVariable(hit.v), Source: varSourceOf(hit.v)})
	}
	return out, nil
}

// SummarizeVariables 批量统计接口的变量使用情况（接口列表徽标）。
// defined = 该接口的请求变量名；undefined = 接口文本中引用但四层都未命中的变量名。
func (s *Service) SummarizeVariables(ctx context.Context, projectID, envID uint64, apiIDs []uint64) (map[uint64]VarSummary, error) {
	out := map[uint64]VarSummary{}
	if projectID == 0 {
		return nil, newBiz(errcode.CodeParamError, "缺少 project_id")
	}
	if len(apiIDs) == 0 {
		return nil, newBiz(errcode.CodeParamError, "缺少 api_ids")
	}
	if len(apiIDs) > maxSummaryApiIDs {
		return nil, newBiz(errcode.CodeParamError, "单次最多查询 %d 个接口，请分批请求", maxSummaryApiIDs)
	}
	apis, err := s.repo.ApisByIDs(ctx, projectID, apiIDs)
	if err != nil {
		return nil, err
	}
	vars, err := s.repo.VariablesForApis(ctx, projectID, apiIDs)
	if err != nil {
		return nil, err
	}
	// 共享层（项目级 + 全局）对所有接口一致；请求级按接口归组。
	shared := map[string]struct{}{}
	byAPI := map[uint64]map[string]struct{}{}
	for _, v := range vars {
		if v.Enabled != model.VarEnabled {
			continue
		}
		if v.Scope == model.VarScopeRequest {
			if byAPI[v.ApiID] == nil {
				byAPI[v.ApiID] = map[string]struct{}{}
			}
			byAPI[v.ApiID][v.Name] = struct{}{}
			continue
		}
		if _, ok := varRank(v, envID); ok {
			shared[v.Name] = struct{}{}
		}
	}
	for _, a := range apis {
		reqVars := byAPI[a.ID]
		defined := make([]string, 0, len(reqVars))
		for name := range reqVars {
			defined = append(defined, name)
		}
		sort.Strings(defined)
		undefined := make([]string, 0)
		for _, name := range apiVarNames(&a) {
			if _, ok := reqVars[name]; ok {
				continue
			}
			if _, ok := shared[name]; ok {
				continue
			}
			undefined = append(undefined, name)
		}
		out[a.ID] = VarSummary{
			Defined:        defined,
			Undefined:      undefined,
			RequestCount:   len(defined),
			UndefinedCount: len(undefined),
		}
	}
	return out, nil
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

// varRank 返回变量在当前环境下的解析优先级（越小越优先）；ok=false 表示不参与解析。
// 优先级表在共享包 varx，本层只负责 model → varx.Var 的映射。
func varRank(v model.Variable, envID uint64) (int, bool) {
	return varx.Rank(toShareVar(v), envID)
}

// toShareVar 把实体映射为共享层变量：项目变量按是否绑定环境区分「当前环境」与「通用」。
func toShareVar(v model.Variable) varx.Var {
	scope := varx.ScopeGlobal
	switch v.Scope {
	case model.VarScopeRequest:
		scope = varx.ScopeRequest
	case model.VarScopeProject:
		scope = varx.ScopeProjectEnv
		if v.EnvID == 0 {
			scope = varx.ScopeProjectCommon
		}
	}
	return varx.Var{
		Name:    v.Name,
		Value:   v.Value,
		Scope:   scope,
		EnvID:   v.EnvID,
		Enabled: v.Enabled == model.VarEnabled,
		Secret:  v.IsSecret == model.VarSecret,
	}
}

// varSourceOf 返回变量来源标记。
func varSourceOf(v model.Variable) string {
	switch v.Scope {
	case model.VarScopeRequest:
		return varSourceRequest
	case model.VarScopeProject:
		return varSourceProject
	}
	return varSourceGlobal
}

// scopeLabel 生成唯一冲突文案里的作用域描述。
func scopeLabel(v *model.Variable) string {
	switch v.Scope {
	case model.VarScopeRequest:
		return "该接口下"
	case model.VarScopeProject:
		if v.EnvID == 0 {
			return "该项目的通用变量中"
		}
		return "该项目环境中"
	}
	return "全局变量中"
}

// maskVariable 实体转视图：敏感值只回掩码并标记是否有值。
func maskVariable(v model.Variable) VarView {
	view := plainVariable(v)
	view.HasValue = v.Value != ""
	// 掩码规则（敏感且有值才掩码）与共享包一致，避免两端各写一套判断。
	view.Value = varx.MaskedValueOf(toShareVar(v))
	return view
}

// plainVariable 实体转视图（值不脱敏，仅用于有写权限的回显路径）。
func plainVariable(v model.Variable) VarView {
	return VarView{
		ID: v.ID, Scope: v.Scope, ProjectID: v.ProjectID, ApiID: v.ApiID, EnvID: v.EnvID,
		Name: v.Name, Value: v.Value, Description: v.Description, IsSecret: v.IsSecret,
		HasValue: v.Value != "", Enabled: v.Enabled, Priority: v.Priority,
		CreateTime: v.CreateTime, UpdateTime: v.UpdateTime,
	}
}

// maskVariables 批量转视图。
func maskVariables(items []model.Variable) []VarView {
	out := make([]VarView, 0, len(items))
	for _, v := range items {
		out = append(out, maskVariable(v))
	}
	return out
}

// derefString 取可选字符串值，nil 视为空串。
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// joinNames 列出前 limit 个名称，超出时追加「等 N 个」。
func joinNames(items []model.Variable, limit int) string {
	names := make([]string, 0, len(items))
	for _, v := range items {
		names = append(names, v.Name)
	}
	if len(names) <= limit {
		return strings.Join(names, "、")
	}
	return strings.Join(names[:limit], "、") + " 等 " + itoa(len(names)) + " 个"
}

// collectVarNames 提取文本中引用的变量名（去重保序，不含内置变量）。
func collectVarNames(text string) []string {
	return varx.Names(text)
}

// apiVarNames 汇总接口在 URL / 请求头 value / 请求参数示例中引用的变量名。
func apiVarNames(a *model.Api) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	add := func(names []string) {
		for _, name := range names {
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	add(collectVarNames(a.URL))
	for _, h := range parseRows(a.HttpRequestHeader) {
		if value, ok := h["value"].(string); ok {
			add(collectVarNames(value))
		}
	}
	for _, p := range parseRows(a.HttpRequestParams) {
		if example, ok := p["example"].(string); ok {
			add(collectVarNames(example))
		}
	}
	return out
}

// encValuePrefix 与客户端 secrets 包的密文前缀逐字一致（enc:v1:）。带该前缀的变量值
// 是客户端口令加密的密文：服务端只归档存储（is_secret 标注加密），**不解析、不解密、
// 不参与渲染/Mock/代码生成**——本产品阶段的既定策略，占位符按「未定义」原样保留。
const encValuePrefix = "enc:v1:"

func isEncryptedValue(v string) bool { return strings.HasPrefix(v, encValuePrefix) }

// resolveTable 返回该接口在当前环境下同名**最高优先级**的变量（保留原文，含敏感值）。
// 仅供服务端渲染 cURL / 代码片段使用；对外接口仍走 ResolveVariables 的脱敏视图。
// 客户端加密的 secret（enc:v1: 前缀）一律跳过——服务端不解析密文，占位符保持原样。
func (s *Service) resolveTable(ctx context.Context, projectID, apiID, envID uint64) (map[string]model.Variable, error) {
	vars, err := s.repo.VariablesForResolve(ctx, projectID, apiID)
	if err != nil {
		return nil, err
	}
	best := map[string]resolvedVar{}
	for _, v := range vars {
		if isEncryptedValue(v.Value) {
			continue
		}
		rank, ok := varRank(v, envID)
		if !ok {
			continue
		}
		if cur, exists := best[v.Name]; !exists || rank < cur.rank {
			best[v.Name] = resolvedVar{v: v, rank: rank}
		}
	}
	table := make(map[string]model.Variable, len(best))
	for name, hit := range best {
		table[name] = hit.v
	}
	return table, nil
}

// renderVars 把文本中的 {{name}} 替换为变量值：未定义保留占位符原文，空值替换为空串。
// table 为空时原样返回（resolve=false 的"保留占位符"模式，连内置变量也不渲染）。
// 表中保留敏感值原文：渲染结果只在服务端使用（cURL/代码片段），单条变量明文不对外回传。
func renderVars(text string, table map[string]model.Variable) string {
	if text == "" || len(table) == 0 {
		return text
	}
	vars := make([]varx.Var, 0, len(table))
	for _, v := range table {
		// 表内已按优先级去重，统一按请求级参与即可保持唯一胜出者。
		sv := toShareVar(v)
		sv.Scope = varx.ScopeRequest
		vars = append(vars, sv)
	}
	out, _ := varx.Build(vars, 0).Resolve(text)
	return out
}

// referencedSecretVars 返回文本引用中命中敏感变量的变量名（去重保序），供导出审计。
func referencedSecretVars(texts []string, table map[string]model.Variable) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, text := range texts {
		for _, name := range collectVarNames(text) {
			hit, ok := table[name]
			if !ok || hit.IsSecret != model.VarSecret {
				continue
			}
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	return out
}
