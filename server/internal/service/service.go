// Package service 业务逻辑层：规则校验、事务编排、调用 repository。对外暴露方法，
// 业务失败以 *BizError 返回以携带错误码，供 handler 直接映射统一响应。
package service

import (
	"context"
	"fmt"

	"apidoc/internal/config"
	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
	"apidoc/internal/repository"
)

// Repository 定义 service 依赖的数据访问接口（便于测试替换）。
type Repository interface {
	CreateUser(ctx context.Context, u *model.UserInfo) error
	UserByID(ctx context.Context, id uint64) (*model.UserInfo, error)
	UserByName(ctx context.Context, name string) (*model.UserInfo, error)
	UserTaken(ctx context.Context, name, email, nick string, excludeID uint64) (bool, error)
	UpdateUser(ctx context.Context, u *model.UserInfo) error
	ListUsers(ctx context.Context, q pageQuery, keyword string) ([]model.UserInfo, int64, error)

	Member(ctx context.Context, userID, projectID uint64) (*model.UserProject, error)
	SaveMember(ctx context.Context, m *model.UserProject) error
	CreateLog(ctx context.Context, v *model.OperationLog) error
}

// pageQuery 与 repository/page.Query 对齐的别名（避免 service 直接依赖包名冲突）。
type pageQuery = struct {
	PageNum  int
	PageSize int
}

// BizError 是带业务错误码的失败。
type BizError struct {
	Code int
	Msg  string
}

// Error 实现 error 接口。
func (e *BizError) Error() string { return e.Msg }

// newBiz 构造业务错误。
func newBiz(code int, format string, args ...any) *BizError {
	return &BizError{Code: code, Msg: fmt.Sprintf(format, args...)}
}

// Service 聚合依赖。
type Service struct {
	repo *repository.Repository
	cfg  *config.Config
}

// New 构造 Service。
func New(repo *repository.Repository, cfg *config.Config) *Service {
	return &Service{repo: repo, cfg: cfg}
}

// recordOp 尽力写操作日志，失败不影响主流程。
func (s *Service) recordOp(ctx context.Context, projectID, userID, objectID uint64, typ int8, action, content string) {
	_ = s.repo.CreateLog(ctx, &model.OperationLog{
		ProjectID: projectID, UserID: userID, ObjectID: objectID, Type: typ, Action: action, Content: content,
	})
}

// isAdmin 判断主体是否管理员/超管（可越过项目成员校验）。
func isAdmin(typ int8) bool {
	return typ == model.UserTypeAdmin || typ == model.UserTypeSuperuser
}

// ensureProjectPermission 校验用户对项目的操作权限；管理员放行。
func (s *Service) ensureProjectPermission(ctx context.Context, userID uint64, typ int8, projectID uint64, need int8) error {
	if isAdmin(typ) {
		return nil
	}
	m, err := s.repo.Member(ctx, userID, projectID)
	if err == repository.ErrNotFound {
		return newBiz(errcode.CodeForbidden, "没有该项目的操作权限")
	}
	if err != nil {
		return err
	}
	if m.IsLeader == 1 || model.Has(m.Permission, need) {
		return nil
	}
	return newBiz(errcode.CodeForbidden, "权限不足")
}
