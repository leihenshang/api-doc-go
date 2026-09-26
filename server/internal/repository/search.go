package repository

import (
	"context"

	"apidoc/internal/model"
)

// ================= 全局搜索 =================
// 搜索范围一律限定在传入的项目 id 集合内（由 service 依据身份计算：管理员全量、其余仅参与的项目）。

// AllProjectIDs 返回全部未删除项目 id（管理员搜索范围）。
func (r *Repository) AllProjectIDs(ctx context.Context) ([]uint64, error) {
	var ids []uint64
	err := r.active(ctx).Model(&model.Project{}).Order("id ASC").Pluck("id", &ids).Error
	return ids, err
}

// MemberProjectIDs 返回用户参与的项目 id（普通用户搜索范围）。
func (r *Repository) MemberProjectIDs(ctx context.Context, userID uint64) ([]uint64, error) {
	var ids []uint64
	err := r.db.WithContext(ctx).Model(&model.UserProject{}).
		Where("user_id = ? AND is_deleted = 0", userID).Order("project_id ASC").Pluck("project_id", &ids).Error
	return ids, err
}

// ProjectsByIDs 取指定 id 的未删除项目。
func (r *Repository) ProjectsByIDs(ctx context.Context, ids []uint64) ([]model.Project, error) {
	var items []model.Project
	if len(ids) == 0 {
		return items, nil
	}
	err := r.active(ctx).Where("id IN ?", ids).Find(&items).Error
	return items, err
}

// SearchProjects 按项目标题搜索。
func (r *Repository) SearchProjects(ctx context.Context, ids []uint64, keyword string, limit int) ([]model.Project, error) {
	var items []model.Project
	if len(ids) == 0 {
		return items, nil
	}
	like := "%" + keyword + "%"
	err := r.active(ctx).Where("id IN ? AND title LIKE ?", ids, like).
		Order("update_time DESC").Limit(limit).Find(&items).Error
	return items, err
}

// SearchApis 在给定项目内按名称 / URL / 说明搜索接口；多取一倍用于「名称命中优先」排序。
func (r *Repository) SearchApis(ctx context.Context, ids []uint64, keyword string, limit int) ([]model.Api, error) {
	var items []model.Api
	if len(ids) == 0 {
		return items, nil
	}
	like := "%" + keyword + "%"
	err := r.active(ctx).Where("project_id IN ?", ids).
		Where("(api_name LIKE ? OR url LIKE ? OR description LIKE ?)", like, like, like).
		Order("update_time DESC").Limit(limit * 2).Find(&items).Error
	return items, err
}

// SearchDocs 在给定项目内按标题 / 正文搜索文档；多取一倍用于「标题命中优先」排序。
func (r *Repository) SearchDocs(ctx context.Context, ids []uint64, keyword string, limit int) ([]model.Doc, error) {
	var items []model.Doc
	if len(ids) == 0 {
		return items, nil
	}
	like := "%" + keyword + "%"
	err := r.active(ctx).Where("project_id IN ?", ids).
		Where("(title LIKE ? OR content LIKE ?)", like, like).
		Order("update_time DESC").Limit(limit * 2).Find(&items).Error
	return items, err
}
