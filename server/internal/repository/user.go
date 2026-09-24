package repository

import (
	"context"
	"errors"
	"strings"

	"apidoc/internal/model"
	"apidoc/internal/pkg/page"

	"gorm.io/gorm"
)

// ErrNotFound 表示记录不存在。
var ErrNotFound = errors.New("repository: not found")

// ---- 用户 ----

// CreateUser 新建用户。
func (r *Repository) CreateUser(ctx context.Context, u *model.UserInfo) error {
	return r.tx(ctx).Create(u).Error
}

// UserByID 按主键取用户。
func (r *Repository) UserByID(ctx context.Context, id uint64) (*model.UserInfo, error) {
	var u model.UserInfo
	if err := r.active(ctx).First(&u, id).Error; err != nil {
		return nil, wrapNotFound(err)
	}
	return &u, nil
}

// UserByName 按登录名取用户（含软删过滤）。
func (r *Repository) UserByName(ctx context.Context, name string) (*model.UserInfo, error) {
	var u model.UserInfo
	if err := r.active(ctx).Where("name = ?", name).First(&u).Error; err != nil {
		return nil, wrapNotFound(err)
	}
	return &u, nil
}

// UserTaken 判断名字/邮箱/昵称是否已被占用（excludeID 排除自身，0 表示不排除）。
// 仅对非空字段做匹配，避免多个空 email 用户互相误判重复。
func (r *Repository) UserTaken(ctx context.Context, name, email, nick string, excludeID uint64) (bool, error) {
	var conds []string
	var args []any
	if name != "" {
		conds, args = append(conds, "name = ?"), append(args, name)
	}
	if email != "" {
		conds, args = append(conds, "email = ?"), append(args, email)
	}
	if nick != "" {
		conds, args = append(conds, "nick_name = ?"), append(args, nick)
	}
	if len(conds) == 0 {
		return false, nil
	}
	q := r.active(ctx).Where(strings.Join(conds, " OR "), args...)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var n int64
	if err := q.Model(&model.UserInfo{}).Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// UpdateUser 保存用户字段变更。
func (r *Repository) UpdateUser(ctx context.Context, u *model.UserInfo) error {
	return r.tx(ctx).Save(u).Error
}

// ListUsers 分页查询用户。
func (r *Repository) ListUsers(ctx context.Context, q page.Query, keyword string) ([]model.UserInfo, int64, error) {
	db := r.active(ctx)
	if keyword != "" {
		like := keyword + "%"
		db = db.Where("name LIKE ? OR nick_name LIKE ?", like, like)
	}
	var total int64
	if err := db.Model(&model.UserInfo{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.UserInfo
	err := db.Order("id DESC").Limit(q.PageSize).Offset(q.Offset()).Find(&items).Error
	return items, total, err
}

// ---- 项目 ----

// CreateProject 新建项目。
func (r *Repository) CreateProject(ctx context.Context, p *model.Project) error {
	return r.tx(ctx).Create(p).Error
}

// ProjectByID 按主键取项目。
func (r *Repository) ProjectByID(ctx context.Context, id uint64) (*model.Project, error) {
	var p model.Project
	if err := r.active(ctx).First(&p, id).Error; err != nil {
		return nil, wrapNotFound(err)
	}
	return &p, nil
}

// UpdateProject 保存项目变更。
func (r *Repository) UpdateProject(ctx context.Context, p *model.Project) error {
	return r.tx(ctx).Save(p).Error
}

// DeleteProject 软删除项目。
func (r *Repository) DeleteProject(ctx context.Context, id uint64) error {
	return r.tx(ctx).Model(&model.Project{}).Where("id = ?", id).Update("is_deleted", 1).Error
}

// ListProjects 列出某用户参与的项目（管理员传 onlyMine=false 看全部）。
func (r *Repository) ListProjects(ctx context.Context, userID uint64, onlyMine bool, q page.Query) ([]model.Project, int64, error) {
	db := r.active(ctx)
	if onlyMine {
		var ids []uint64
		if err := r.tx(ctx).Model(&model.UserProject{}).Where("user_id = ? AND is_deleted = 0", userID).
			Pluck("project_id", &ids).Error; err != nil {
			return nil, 0, err
		}
		db = db.Where("id IN ?", ids)
	}
	var total int64
	if err := db.Model(&model.Project{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.Project
	err := db.Order("id DESC").Limit(q.PageSize).Offset(q.Offset()).Find(&items).Error
	return items, total, err
}

// ---- 项目成员 ----

// AddMember 添加成员关系。
func (r *Repository) AddMember(ctx context.Context, m *model.UserProject) error {
	return r.tx(ctx).Create(m).Error
}

// Member 取某用户在某项目的成员记录。
func (r *Repository) Member(ctx context.Context, userID, projectID uint64) (*model.UserProject, error) {
	var m model.UserProject
	err := r.active(ctx).Where("user_id = ? AND project_id = ?", userID, projectID).First(&m).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &m, nil
}

// ListMembers 列出项目成员。
func (r *Repository) ListMembers(ctx context.Context, projectID uint64) ([]model.UserProject, error) {
	var items []model.UserProject
	err := r.active(ctx).Where("project_id = ?", projectID).Find(&items).Error
	return items, err
}

// SaveMember 保存成员记录变更。
func (r *Repository) SaveMember(ctx context.Context, m *model.UserProject) error {
	return r.tx(ctx).Save(m).Error
}

// wrapNotFound 将 gorm 未找到错误归一为 ErrNotFound。
func wrapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
