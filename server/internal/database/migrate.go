package database

import (
	"fmt"

	"apidoc/internal/model"

	"gorm.io/gorm"
)

// Migrate 依据实体定义自动建表/加列（GORM AutoMigrate），方言无关。
// 生产环境可另用 migrations/ 下的 SQL 脚本；此处保证开发期开箱可用。
func Migrate(db *gorm.DB) error {
	entities := []any{
		&model.UserInfo{},
		&model.Project{},
		&model.UserProject{},
		&model.Group{},
		&model.Api{},
		&model.Doc{},
		&model.Bug{},
		&model.BugAssign{},
		&model.FieldMapping{},
		&model.Property{},
		&model.Message{},
		&model.Notification{},
		&model.FileRecord{},
		&model.DatabaseConn{},
		&model.OperationLog{},
	}
	if err := db.AutoMigrate(entities...); err != nil {
		return fmt.Errorf("database: automigrate: %w", err)
	}
	return nil
}
