package repository

import (
	"context"

	"apidoc/internal/model"
)

// ================= 环境 =================

func (r *Repository) CreateVariableEnv(ctx context.Context, v *model.VariableEnv) error {
	return gmCreate(ctx, r.db, v)
}
func (r *Repository) GetVariableEnv(ctx context.Context, id uint64) (*model.VariableEnv, error) {
	return gmGet[model.VariableEnv](ctx, r.db, id)
}
func (r *Repository) UpdateVariableEnv(ctx context.Context, v *model.VariableEnv) error {
	return gmSave(ctx, r.db, v)
}
func (r *Repository) DeleteVariableEnv(ctx context.Context, id uint64) error {
	return gmSoftDelete[model.VariableEnv](ctx, r.db, id)
}

// ListVariableEnvs 返回项目下的环境，默认环境优先，其次按 priority/id。
func (r *Repository) ListVariableEnvs(ctx context.Context, projectID uint64) ([]model.VariableEnv, error) {
	var items []model.VariableEnv
	err := r.active(ctx).Where("project_id = ?", projectID).
		Order("is_default DESC, priority ASC, id ASC").Find(&items).Error
	return items, err
}

// VariableEnvTaken 判断项目内是否已存在同名未删除环境；excludeID 用于排除自身。
func (r *Repository) VariableEnvTaken(ctx context.Context, projectID uint64, name string, excludeID uint64) (bool, error) {
	db := r.active(ctx).Model(&model.VariableEnv{}).Where("project_id = ? AND name = ?", projectID, name)
	if excludeID > 0 {
		db = db.Where("id <> ?", excludeID)
	}
	var count int64
	if err := db.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// CountVariableEnvs 统计项目下的环境数量（首个环境自动置为默认）。
func (r *Repository) CountVariableEnvs(ctx context.Context, projectID uint64) (int64, error) {
	return gmCount[model.VariableEnv](r.active(ctx).Where("project_id = ?", projectID))
}

// ClearVariableEnvDefault 清除项目内其它环境的默认标记（exceptID 为即将成为默认的环境）。
func (r *Repository) ClearVariableEnvDefault(ctx context.Context, projectID, exceptID uint64) error {
	return r.db.WithContext(ctx).Model(&model.VariableEnv{}).
		Where("project_id = ? AND id <> ? AND is_default = 1", projectID, exceptID).
		Update("is_default", model.VarEnvNotDefault).Error
}

// ================= 变量 =================

func (r *Repository) CreateVariable(ctx context.Context, v *model.Variable) error {
	return gmCreate(ctx, r.db, v)
}
func (r *Repository) GetVariable(ctx context.Context, id uint64) (*model.Variable, error) {
	return gmGet[model.Variable](ctx, r.db, id)
}
func (r *Repository) UpdateVariable(ctx context.Context, v *model.Variable) error {
	return gmSave(ctx, r.db, v)
}
func (r *Repository) DeleteVariable(ctx context.Context, id uint64) error {
	return gmSoftDelete[model.Variable](ctx, r.db, id)
}

// VariableFilter 变量列表筛选（零值表示不筛选；EnvID 为指针以便区分「不限」与「通用(0)」）。
type VariableFilter struct {
	ProjectID uint64
	Scope     int8
	ApiID     uint64
	EnvID     *uint64
	Keyword   string
}

// ListVariables 按筛选条件返回变量：
// scope 指定时只看该作用域（全局变量不看项目）；未指定时返回「本项目 + 全局」。
// 关键词只匹配名称与描述（不匹配值，避免探测敏感值）。
func (r *Repository) ListVariables(ctx context.Context, f VariableFilter) ([]model.Variable, error) {
	db := r.active(ctx).Model(&model.Variable{})
	switch {
	case f.Scope == model.VarScopeGlobal:
		db = db.Where("scope = ?", model.VarScopeGlobal)
	case f.Scope > 0:
		db = db.Where("scope = ?", f.Scope)
		if f.ProjectID > 0 {
			db = db.Where("project_id = ?", f.ProjectID)
		}
	default:
		if f.ProjectID > 0 {
			db = db.Where("(project_id = ? OR scope = ?)", f.ProjectID, model.VarScopeGlobal)
		}
	}
	if f.ApiID > 0 {
		db = db.Where("api_id = ?", f.ApiID)
	}
	if f.EnvID != nil {
		db = db.Where("env_id = ?", *f.EnvID)
	}
	if f.Keyword != "" {
		like := "%" + f.Keyword + "%"
		db = db.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	var items []model.Variable
	err := db.Order("scope ASC, env_id ASC, priority ASC, id ASC").Find(&items).Error
	return items, err
}

// VariableTaken 判断同层同环境下是否已存在同名未删除变量；excludeID 用于排除自身。
func (r *Repository) VariableTaken(ctx context.Context, scope int8, projectID, apiID, envID uint64, name string, excludeID uint64) (bool, error) {
	db := r.active(ctx).Model(&model.Variable{}).
		Where("scope = ? AND project_id = ? AND api_id = ? AND env_id = ? AND name = ?",
			scope, projectID, apiID, envID, name)
	if excludeID > 0 {
		db = db.Where("id <> ?", excludeID)
	}
	var count int64
	if err := db.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// VariablesByEnv 返回绑定某环境的全部未删除变量（环境删除前的引用检查）。
func (r *Repository) VariablesByEnv(ctx context.Context, envID uint64) ([]model.Variable, error) {
	var items []model.Variable
	err := r.active(ctx).Where("env_id = ?", envID).Order("name ASC").Find(&items).Error
	return items, err
}

// VariablesForResolve 返回解析某接口所需的全部变量：全局 + 本项目 + 该接口的请求级变量。
func (r *Repository) VariablesForResolve(ctx context.Context, projectID, apiID uint64) ([]model.Variable, error) {
	var items []model.Variable
	err := r.active(ctx).
		Where("scope = ? OR (scope = ? AND project_id = ?) OR (scope = ? AND api_id = ?)",
			model.VarScopeGlobal, model.VarScopeProject, projectID, model.VarScopeRequest, apiID).
		Find(&items).Error
	return items, err
}

// VariablesForApis 批量解析用：全局 + 本项目 + 指定接口集合的请求级变量。
func (r *Repository) VariablesForApis(ctx context.Context, projectID uint64, apiIDs []uint64) ([]model.Variable, error) {
	var items []model.Variable
	if len(apiIDs) == 0 {
		return items, nil
	}
	err := r.active(ctx).
		Where("scope = ? OR (scope = ? AND project_id = ?) OR (scope = ? AND api_id IN ?)",
			model.VarScopeGlobal, model.VarScopeProject, projectID, model.VarScopeRequest, apiIDs).
		Find(&items).Error
	return items, err
}
