package service

import (
	"context"
	"fmt"

	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
	"apidoc/internal/pkg/page"
	"apidoc/internal/repository"
)

// MemberView 是成员列表视图（含用户基本信息）。
type MemberView struct {
	UserID     uint64 `json:"user_id"`
	Name       string `json:"name"`
	NickName   string `json:"nick_name"`
	IsLeader   int8   `json:"is_leader"`
	Permission int8   `json:"permission"`
}

// CreateProject 创建项目并把创建者设为负责人。
func (s *Service) CreateProject(ctx context.Context, creatorID uint64, req dto.ProjectCreateReq) (*model.Project, error) {
	if err := s.ensureTitleFree(ctx, req.Title, 0); err != nil {
		return nil, err
	}
	p := &model.Project{Title: req.Title, Description: req.Description, Version: req.Version, Type: req.Type}
	if err := s.repo.CreateProject(ctx, p); err != nil {
		// 并发同名由唯一索引兜底，保留原始错误供排查
		return nil, wrapBiz(errcode.CodeFailed, err, "创建项目失败，请稍后重试")
	}
	if err := s.repo.AddMember(ctx, &model.UserProject{
		UserID: creatorID, ProjectID: p.ID, IsLeader: 1, Permission: model.PermAll,
	}); err != nil {
		return nil, err
	}
	s.recordOp(ctx, p.ID, creatorID, p.ID, model.LogObjectProject, "create", "项目:"+p.Title)
	return p, nil
}

// UpdateProject 更新项目。
func (s *Service) UpdateProject(ctx context.Context, userID uint64, typ int8, req dto.ProjectUpdateReq) (*model.Project, error) {
	if err := s.ensureProjectPermission(ctx, userID, typ, req.ID, model.PermUpdate); err != nil {
		return nil, err
	}
	p, err := s.repo.ProjectByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if req.Title != "" && req.Title != p.Title {
		if err := s.ensureTitleFree(ctx, req.Title, p.ID); err != nil {
			return nil, err
		}
		p.Title = req.Title
	}
	p.Description = req.Description
	p.Version = req.Version
	p.Type = req.Type
	if req.ClientWriteEnabled != nil {
		p.ClientWriteEnabled = req.ClientWriteEnabled
	}
	if err := s.repo.UpdateProject(ctx, p); err != nil {
		return nil, wrapBiz(errcode.CodeFailed, err, "保存项目失败，请稍后重试")
	}
	s.recordOp(ctx, p.ID, userID, p.ID, model.LogObjectProject, "update", "项目:"+p.Title)
	return p, nil
}

// ensureTitleFree 校验项目名称未被其它未删除项目占用（软删除的项目不占用名称）。
func (s *Service) ensureTitleFree(ctx context.Context, title string, excludeID uint64) error {
	taken, err := s.repo.ProjectTitleTaken(ctx, title, excludeID)
	if err != nil {
		return fmt.Errorf("service: 校验项目名称: %w", err)
	}
	if taken {
		return newBiz(errcode.CodeFailed, "项目名称「%s」已存在，请更换名称", title)
	}
	return nil
}

// GetProject 取项目详情。
func (s *Service) GetProject(ctx context.Context, id uint64) (*model.Project, error) {
	return s.repo.ProjectByID(ctx, id)
}

// ListProjects 列出项目；管理员看全部，普通用户看自己参与的。
func (s *Service) ListProjects(ctx context.Context, userID uint64, typ int8, q page.Query) (page.Result[model.Project], error) {
	q.Normalize()
	items, total, err := s.repo.ListProjects(ctx, userID, !isAdmin(typ), q)
	if err != nil {
		return page.Result[model.Project]{}, err
	}
	return page.NewResult(items, total, q), nil
}

// DeleteProject 软删除项目。
func (s *Service) DeleteProject(ctx context.Context, userID uint64, typ int8, id uint64) error {
	if err := s.ensureProjectPermission(ctx, userID, typ, id, model.PermDelete); err != nil {
		return err
	}
	return s.repo.DeleteProject(ctx, id)
}

// AddMember 添加成员。需项目管理员或负责人。
func (s *Service) AddMember(ctx context.Context, operatorID uint64, typ int8, req dto.MemberAddReq) error {
	if err := s.ensureManage(ctx, operatorID, typ, req.ProjectID); err != nil {
		return err
	}
	if _, err := s.repo.UserByID(ctx, req.UserID); err != nil {
		return newBiz(errcode.CodeParamError, "用户不存在")
	}
	m, err := s.repo.Member(ctx, req.UserID, req.ProjectID)
	if err == repository.ErrNotFound {
		perm := req.Permission
		if perm == 0 {
			perm = model.PermReadonly
		}
		return s.repo.AddMember(ctx, &model.UserProject{
			UserID: req.UserID, ProjectID: req.ProjectID, Permission: perm,
		})
	}
	if err != nil {
		return err
	}
	m.IsDeleted = model.IsDeletedNo
	return s.repo.SaveMember(ctx, m)
}

// ListMembers 列出项目成员。
func (s *Service) ListMembers(ctx context.Context, projectID uint64) ([]MemberView, error) {
	members, err := s.repo.ListMembers(ctx, projectID)
	if err != nil {
		return nil, err
	}
	views := make([]MemberView, 0, len(members))
	for _, m := range members {
		v := MemberView{UserID: m.UserID, IsLeader: m.IsLeader, Permission: m.Permission}
		if u, err := s.repo.UserByID(ctx, m.UserID); err == nil {
			v.Name, v.NickName = u.Name, u.NickName
		}
		views = append(views, v)
	}
	return views, nil
}

// SetLeader 设置项目负责人。
func (s *Service) SetLeader(ctx context.Context, operatorID uint64, typ int8, req dto.SetLeaderReq) error {
	if err := s.ensureManage(ctx, operatorID, typ, req.ProjectID); err != nil {
		return err
	}
	m, err := s.repo.Member(ctx, req.UserID, req.ProjectID)
	if err != nil {
		return newBiz(errcode.CodeParamError, "该用户不是项目成员")
	}
	m.IsLeader = 1
	m.Permission = model.PermAll
	return s.repo.SaveMember(ctx, m)
}

// SetPermission 设置成员权限位。
func (s *Service) SetPermission(ctx context.Context, operatorID uint64, typ int8, req dto.SetPermissionReq) error {
	if err := s.ensureManage(ctx, operatorID, typ, req.ProjectID); err != nil {
		return err
	}
	m, err := s.repo.Member(ctx, req.UserID, req.ProjectID)
	if err != nil {
		return newBiz(errcode.CodeParamError, "该用户不是项目成员")
	}
	m.Permission = req.Permission
	return s.repo.SaveMember(ctx, m)
}

// QuitProject 退出项目。
func (s *Service) QuitProject(ctx context.Context, userID uint64, req dto.QuitProjectReq) error {
	m, err := s.repo.Member(ctx, userID, req.ProjectID)
	if err != nil {
		return newBiz(errcode.CodeParamError, "你不在该项目中")
	}
	m.IsDeleted = model.IsDeletedYes
	return s.repo.SaveMember(ctx, m)
}

// MyPermission 返回当前用户在项目中的权限掩码（管理员返回全权限）。
func (s *Service) MyPermission(ctx context.Context, userID uint64, typ int8, projectID uint64) int8 {
	if isAdmin(typ) {
		return model.PermAll
	}
	if m, err := s.repo.Member(ctx, userID, projectID); err == nil {
		if m.IsLeader == 1 {
			return model.PermAll
		}
		return m.Permission
	}
	return 0
}

// ensureManage 校验调用者可管理成员（管理员或项目负责人）。
func (s *Service) ensureManage(ctx context.Context, userID uint64, typ int8, projectID uint64) error {
	if isAdmin(typ) {
		return nil
	}
	m, err := s.repo.Member(ctx, userID, projectID)
	if err == repository.ErrNotFound {
		return newBiz(errcode.CodeForbidden, "没有该项目的管理权限")
	}
	if err != nil {
		return err
	}
	if m.IsLeader != 1 {
		return newBiz(errcode.CodeForbidden, "仅项目负责人可管理成员")
	}
	return nil
}
