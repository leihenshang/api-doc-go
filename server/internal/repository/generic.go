package repository

import (
	"context"

	"gorm.io/gorm"
)

// 通用 CRUD 助手，减少各实体仓储样板。
// 注意：软删除已移除（2026-09-30），删除即硬删除；
// 需要「恢复」时走历史记录（change_log）重建，不再有回收站/restore/purge。
// is_deleted 列保留兼容旧数据，但不再写入，查询也不再过滤。

// gmCreate 新建记录。
func gmCreate[T any](ctx context.Context, db *gorm.DB, v *T) error {
	return db.WithContext(ctx).Create(v).Error
}

// gmGet 按主键取记录。
func gmGet[T any](ctx context.Context, db *gorm.DB, id uint64) (*T, error) {
	var v T
	if err := db.WithContext(ctx).First(&v, id).Error; err != nil {
		return nil, wrapNotFound(err)
	}
	return &v, nil
}

// gmSave 保存整条记录。
func gmSave[T any](ctx context.Context, db *gorm.DB, v *T) error {
	return db.WithContext(ctx).Save(v).Error
}

// gmUpdateSort 仅更新排序相关列：parentCol 为父级列名（p_id / group_id），priority 为同级顺序。
func gmUpdateSort[T any](ctx context.Context, db *gorm.DB, id, parentID uint64, parentCol string, priority int) error {
	return db.WithContext(ctx).Model(new(T)).Where("id = ?", id).
		Updates(map[string]any{parentCol: parentID, "priority": priority}).Error
}

// gmHardDelete 物理删除（软删除已移除）。
func gmHardDelete[T any](ctx context.Context, db *gorm.DB, id uint64) error {
	return db.WithContext(ctx).Delete(new(T), id).Error
}

// gmCount 统计满足作用域的记录数。
func gmCount[T any](scoped *gorm.DB) (int64, error) {
	var n int64
	err := scoped.Model(new(T)).Count(&n).Error
	return n, err
}
