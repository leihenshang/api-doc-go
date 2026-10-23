package repository

// 内容快照数据访问（RM-05）：只做 CRUD 与按实体倒序查询，diff/回滚规则在 service。

import (
	"context"

	"apidoc/internal/model"
)

// snapshotKeep 每个实体保留的最近快照数（超出按 revision 从旧到新删除）。
const snapshotKeep = 50

// CreateSnapshot 写入一份快照，并裁剪该实体的历史条数。
func (r *Repository) CreateSnapshot(ctx context.Context, s *model.ContentSnapshot) error {
	if err := gmCreate(ctx, r.db, s); err != nil {
		return err
	}
	var old []uint64
	if err := r.tx(ctx).Model(&model.ContentSnapshot{}).
		Where("entity_type = ? AND entity_id = ?", s.EntityType, s.EntityID).
		Order("revision DESC, id DESC").Offset(snapshotKeep).Limit(1000).
		Pluck("id", &old).Error; err != nil {
		return err
	}
	if len(old) > 0 {
		return r.tx(ctx).Delete(&model.ContentSnapshot{}, old).Error
	}
	return nil
}

// ListSnapshots 取实体快照（revision 倒序），limit <= 0 表示默认 50。
func (r *Repository) ListSnapshots(ctx context.Context, entityType string, entityID uint64, limit int) ([]model.ContentSnapshot, error) {
	if limit <= 0 {
		limit = snapshotKeep
	}
	var items []model.ContentSnapshot
	err := r.tx(ctx).Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Order("revision DESC, id DESC").Limit(limit).Find(&items).Error
	return items, err
}

// SnapshotByID 按主键取快照（含完整 payload）。
func (r *Repository) SnapshotByID(ctx context.Context, id uint64) (*model.ContentSnapshot, error) {
	return gmGet[model.ContentSnapshot](ctx, r.db, id)
}
