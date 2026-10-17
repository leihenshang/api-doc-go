// Package repository 数据访问层：仅封装 GORM 查询与写库，不含业务规则。
package repository

import (
	"context"

	"apidoc/internal/config"

	"gorm.io/gorm"
)

// Repository 封装各实体的数据访问。
type Repository struct {
	db  *gorm.DB
	cfg *config.Config
}

// New 构造 Repository。
func New(db *gorm.DB, cfg *config.Config) *Repository {
	return &Repository{db: db, cfg: cfg}
}

// DB 暴露底层句柄，供 service 组织事务。
func (r *Repository) DB() *gorm.DB { return r.db }

// Transaction 在单个事务内执行 fn；fn 拿到的是绑定该事务的 Repository 副本，
// 其上的读写（含 AppendChange/SetLastChange）共享同一 tx，任一失败整体回滚。
// 用于「多条写必须同生共死」的场景（如建项目 + 加成员、覆盖导入）。
func (r *Repository) Transaction(ctx context.Context, fn func(tx *Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&Repository{db: tx, cfg: r.cfg})
	})
}

// tx 返回带上下文的会话。
func (r *Repository) tx(ctx context.Context) *gorm.DB { return r.db.WithContext(ctx) }

// active 排除软删除记录。
func (r *Repository) active(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Where("is_deleted = ?", 0)
}
