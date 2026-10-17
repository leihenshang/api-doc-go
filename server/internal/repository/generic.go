package repository

import (
	"context"

	"gorm.io/gorm"
)

// 通用 CRUD 助手，减少各实体仓储样板。
// 删除语义：api / doc / group 走软删除（is_deleted=1，进回收站，可 restore / purge）；
// 其余实体（bug / 字段映射 / 属性 / 分享 / 变量 / 响应示例）维持物理删除，不在回收站范围。
// 读路径用 getActive（排除已删除），回收站与还原路径用 gmGet / *ByIDsAny（含已删除）。

// gmCreate 新建记录。
func gmCreate[T any](ctx context.Context, db *gorm.DB, v *T) error {
	return db.WithContext(ctx).Create(v).Error
}

// gmGet 按主键取记录（**含已软删除**）。回收站还原/彻底删除需读到 is_deleted=1 的行，
// 故这里不做过滤；业务读路径一律用 getActive。
func gmGet[T any](ctx context.Context, db *gorm.DB, id uint64) (*T, error) {
	var v T
	if err := db.WithContext(ctx).First(&v, id).Error; err != nil {
		return nil, wrapNotFound(err)
	}
	return &v, nil
}

// getActive 按主键取未删除记录。已软删除的行视同不存在。
func getActive[T any](ctx context.Context, db *gorm.DB, id uint64) (*T, error) {
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
	return db.WithContext(ctx).Model(new(T)).Where("id = ?", id).
		Updates(map[string]any{parentCol: parentID, "priority": priority}).Error
}

// gmHardDelete 物理删除。仅用于不在回收站范围内的实体（bug / 字段映射 / 属性 / 分享 / 变量 / 响应示例）。
func gmHardDelete[T any](ctx context.Context, db *gorm.DB, id uint64) error {
	return db.WithContext(ctx).Delete(new(T), id).Error
}

// gmSoftDelete 软删除进回收站：置 is_deleted=1，行保留在库中，可由 gmRestore 还原。
func gmSoftDelete[T any](ctx context.Context, db *gorm.DB, id uint64) error {
	return db.WithContext(ctx).Model(new(T)).Where("id = ?", id).
		Update("is_deleted", 1).Error
}

// gmRestore 从回收站还原：置 is_deleted=0。父级已不存在的场景由 service 层负责回溯补齐。
func gmRestore[T any](ctx context.Context, db *gorm.DB, id uint64) error {
	return db.WithContext(ctx).Model(new(T)).Where("id = ?", id).
		Update("is_deleted", 0).Error
}

// gmPurge 彻底删除：回收站中再删一次即物理移除，不可恢复。
func gmPurge[T any](ctx context.Context, db *gorm.DB, id uint64) error {
	return db.WithContext(ctx).Delete(new(T), id).Error
}

// gmSoftDeleteByIDs 批量软删除，返回实际影响行数（已删除的行不计入）。
func gmSoftDeleteByIDs[T any](ctx context.Context, db *gorm.DB, ids []uint64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	res := db.WithContext(ctx).Model(new(T)).Where("id IN ?", ids).Update("is_deleted", 1)
	return res.RowsAffected, res.Error
}

// gmRestoreByIDs 批量还原，返回实际影响行数（本就未删除的行不计入）。
func gmRestoreByIDs[T any](ctx context.Context, db *gorm.DB, ids []uint64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	res := db.WithContext(ctx).Model(new(T)).Where("id IN ?", ids).Update("is_deleted", 0)
	return res.RowsAffected, res.Error
}

// gmPurgeByIDs 批量彻底删除，返回实际删除行数。
func gmPurgeByIDs[T any](ctx context.Context, db *gorm.DB, ids []uint64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	res := db.WithContext(ctx).Delete(new(T), ids)
	return res.RowsAffected, res.Error
}

// gmCount 统计满足作用域的记录数。
func gmCount[T any](scoped *gorm.DB) (int64, error) {
	var n int64
	err := scoped.Model(new(T)).Count(&n).Error
	return n, err
}
