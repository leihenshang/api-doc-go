package repository

import (
	"context"

	"gorm.io/gorm"
)

// 通用 CRUD 助手，减少各实体仓储样板。软删除依赖列 is_deleted（0/1）。

// gmCreate 新建记录。
func gmCreate[T any](ctx context.Context, db *gorm.DB, v *T) error {
	return db.WithContext(ctx).Create(v).Error
}

// gmGet 按主键取未软删除记录。
func gmGet[T any](ctx context.Context, db *gorm.DB, id uint64) (*T, error) {
	var v T
	if err := db.WithContext(ctx).Where("is_deleted = ?", 0).First(&v, id).Error; err != nil {
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
	return db.WithContext(ctx).Model(new(T)).Where("id = ? AND is_deleted = 0", id).
		Updates(map[string]any{parentCol: parentID, "priority": priority}).Error
}

// gmSoftDelete 标记软删除。
func gmSoftDelete[T any](ctx context.Context, db *gorm.DB, id uint64) error {
	return db.WithContext(ctx).Model(new(T)).Where("id = ?", id).Update("is_deleted", 1).Error
}

// gmRestore 撤销软删除。
func gmRestore[T any](ctx context.Context, db *gorm.DB, id uint64) error {
	return db.WithContext(ctx).Model(new(T)).Where("id = ?", id).Update("is_deleted", 0).Error
}

// gmRestoreMany 批量撤销软删除，返回受影响行数。
func gmRestoreMany[T any](ctx context.Context, db *gorm.DB, ids []uint64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	tx := db.WithContext(ctx).Model(new(T)).Where("id IN ?", ids).Update("is_deleted", 0)
	return tx.RowsAffected, tx.Error
}

// gmPurgeMany 批量**彻底删除**（回收站清理，绕过软删除），返回受影响行数。
func gmPurgeMany[T any](ctx context.Context, db *gorm.DB, ids []uint64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	tx := db.WithContext(ctx).Unscoped().Where("id IN ?", ids).Delete(new(T))
	return tx.RowsAffected, tx.Error
}

// gmCount 统计满足作用域的记录数。
func gmCount[T any](scoped *gorm.DB) (int64, error) {
	var n int64
	err := scoped.Model(new(T)).Count(&n).Error
	return n, err
}
