package service

// 变量环境（P2-14：从 variable.go 拆出）：环境是变量的容器，先于变量同步。

import (
	"context"
	"strings"

	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
)

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
	// P0-7 深层原因：Web 新建也要分配 ext_uid，否则变更流水与客户端同步都拿不到身份
	env.ExtUID = ensureExtUID("")
	if n == 0 {
		env.IsDefault = model.VarEnvDefault
	}
	if err := s.repo.CreateVariableEnv(ctx, env); err != nil {
		return nil, err
	}
	s.recordOp(ctx, env.ProjectID, userID, env.ID, model.LogObjectVariableEnv, "create", "环境:"+env.Name)
	s.logVarChange(ctx, env.ProjectID, model.SyncTypeVarEnv, env.ID, env.ExtUID, "upsert", userID)
	return env, nil
}

// ListVariableEnvs 列出项目环境（默认环境优先）。P0-1：需项目读权限。
func (s *Service) ListVariableEnvs(ctx context.Context, userID uint64, typ int8, projectID uint64) ([]model.VariableEnv, error) {
	if projectID == 0 {
		return nil, newBiz(errcode.CodeParamError, "缺少 project_id")
	}
	if err := s.ensureProjectPermission(ctx, userID, typ, projectID, model.PermQuery); err != nil {
		return nil, err
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
	s.logVarChange(ctx, env.ProjectID, model.SyncTypeVarEnv, env.ID, env.ExtUID, "upsert", userID)
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
	s.logVarChange(ctx, env.ProjectID, model.SyncTypeVarEnv, env.ID, env.ExtUID, "upsert", userID)
	return nil
}

// VariableEnvRefs 返回引用某环境的资源明细（删除前预检）。
func (s *Service) VariableEnvRefs(ctx context.Context, userID uint64, typ int8, id uint64) (*EnvRefs, error) {
	env, err := s.repo.GetVariableEnv(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.ensureProjectPermission(ctx, userID, typ, env.ProjectID, model.PermQuery); err != nil {
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
	s.logVarChange(ctx, env.ProjectID, model.SyncTypeVarEnv, env.ID, env.ExtUID, "delete", userID)
	return nil
}

// ================= 变量 =================
