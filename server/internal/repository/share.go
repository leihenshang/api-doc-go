package repository

import (
	"context"

	"apidoc/internal/model"
)

// ================= 分享链接 =================

func (r *Repository) CreateShareLink(ctx context.Context, v *model.ShareLink) error {
	return gmCreate(ctx, r.db, v)
}
func (r *Repository) ShareLinkByID(ctx context.Context, id uint64) (*model.ShareLink, error) {
	return gmGet[model.ShareLink](ctx, r.db, id)
}
func (r *Repository) DeleteShareLink(ctx context.Context, id uint64) error {
	return gmHardDelete[model.ShareLink](ctx, r.db, id)
}

// ListShareLinks 项目下的分享链接（新建在前）。
func (r *Repository) ListShareLinks(ctx context.Context, projectID uint64) ([]model.ShareLink, error) {
	var items []model.ShareLink
	err := r.active(ctx).Where("project_id = ?", projectID).Order("id DESC").Find(&items).Error
	return items, err
}

// ShareLinkByToken 按令牌取分享链接（公开只读访问用）。
func (r *Repository) ShareLinkByToken(ctx context.Context, token string) (*model.ShareLink, error) {
	var v model.ShareLink
	if err := r.active(ctx).Where("token = ?", token).First(&v).Error; err != nil {
		return nil, wrapNotFound(err)
	}
	return &v, nil
}

// DocsByProject 取项目下全部未删除文档（分享只读视图用，不分页）。
func (r *Repository) DocsByProject(ctx context.Context, projectID uint64) ([]model.Doc, error) {
	var items []model.Doc
	err := r.active(ctx).Where("project_id = ?", projectID).Order("is_top DESC, priority ASC, id ASC").Find(&items).Error
	return items, err
}
