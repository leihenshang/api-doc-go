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

// gmSoftDelete 标记软删除。
func gmSoftDelete[T any](ctx context.Context, db *gorm.DB, id uint64) error {
	return db.WithContext(ctx).Model(new(T)).Where("id = ?", id).Update("is_deleted", 1).Error
}

// gmRestore 撤销软删除。
func gmRestore[T any](ctx context.Context, db *gorm.DB, id uint64) error {
	return db.WithContext(ctx).Model(new(T)).Where("id = ?", id).Update("is_deleted", 0).Error
}

// gmCount 统计满足作用域的记录数。
func gmCount[T any](scoped *gorm.DB) (int64, error) {
	var n int64
	err := scoped.Model(new(T)).Count(&n).Error
	return n, err
}
