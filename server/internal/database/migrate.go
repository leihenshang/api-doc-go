package database

import (
	"fmt"

	"apidoc/internal/model"

	"gorm.io/gorm"
)

// legacyProjectTitleIndex 是 project.title 的历史全局唯一索引（不区分软删除），
// 会让软删除后的项目名被永久占用，表现为「列表里没有同名项目却创建失败」。
const legacyProjectTitleIndex = "idx_project_title"

// Migrate 依据实体定义自动建表/加列（GORM AutoMigrate），并修正历史索引。
// 生产环境可另用 migrations/ 下的 SQL 脚本；此处保证开发期开箱可用。
func Migrate(db *gorm.DB) error {
	if err := dropLegacyProjectTitleIndex(db); err != nil {
		return err
	}
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
		&model.VariableEnv{},
		&model.Variable{},
		&model.ShareLink{},
		&model.Property{},
		&model.Message{},
		&model.Notification{},
		&model.FileRecord{},
		&model.OperationLog{},
	}
	if err := db.AutoMigrate(entities...); err != nil {
		return fmt.Errorf("database: automigrate: %w", err)
	}
	return nil
}

// dropLegacyProjectTitleIndex 删除历史全局唯一索引，改由 uk_project_title 局部唯一索引接管。
// 仅 sqlite/postgres 支持 IF EXISTS 与局部索引，其它方言保持原状以免误删索引。
func dropLegacyProjectTitleIndex(db *gorm.DB) error {
	switch db.Dialector.Name() {
	case "sqlite", "postgres":
		if err := db.Exec("DROP INDEX IF EXISTS " + legacyProjectTitleIndex).Error; err != nil {
			return fmt.Errorf("database: drop legacy index %s: %w", legacyProjectTitleIndex, err)
		}
	}
	return nil
}
