package database

import (
	"fmt"

	"apidoc/internal/model"

	"gorm.io/gorm"
)

// legacyProjectTitleIndex 是 project.title 的历史全局唯一索引（不区分软删除），
// 会让软删除后的项目名被永久占用，表现为「列表里没有同名项目却创建失败」。
const legacyProjectTitleIndex = "idx_project_title"

// legacyUserNameEmailIndex 是 user_info 的历史全局唯一索引（不区分软删除）。
// 保留它会让软删用户的 name/email 永久占位：应用层 UserTaken 走 active() 判定为「可用」，
// 插入却在 DB 约束上失败，两边结论互相矛盾。新索引改名为 uk_name_email_active（带 is_deleted = 0）。
const legacyUserNameEmailIndex = "uk_name_email"

// Migrate 依据实体定义自动建表/加列（GORM AutoMigrate），并修正历史索引。
// 生产环境可另用 migrations/ 下的 SQL 脚本；此处保证开发期开箱可用。
func Migrate(db *gorm.DB) error {
	if err := dropLegacyIndexes(db); err != nil {
		return err
	}
	entities := []any{
		&model.UserInfo{},
		&model.Project{},
		&model.UserProject{},
		&model.Group{},
		&model.Api{},
		&model.ApiReturnSample{},
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
		// 同步地基（S1/S3/S4/S7）
		&model.ChangeLog{},
		&model.SyncOp{},
		&model.UserToken{},
	}
	if err := db.AutoMigrate(entities...); err != nil {
		return fmt.Errorf("database: automigrate: %w", err)
	}
	return nil
}

// dropLegacyIndexes 删除历史遗留的「不区分软删除」全局唯一索引，
// 由 AutoMigrate 按 model 的 `where:` tag 重建为局部唯一索引。
// 仅 sqlite/postgres 支持 IF EXISTS 与局部索引，其它方言保持原状以免误删索引。
//
// 每项列名都与新索引名不同，所以重复启动是幂等的：第一次删掉旧名，
// 之后每次 DROP IF EXISTS 都无事发生，不会再触发一次「删了又建」。
func dropLegacyIndexes(db *gorm.DB) error {
	switch db.Dialector.Name() {
	case "sqlite", "postgres":
		for _, name := range []string{legacyProjectTitleIndex, legacyUserNameEmailIndex} {
			if err := db.Exec("DROP INDEX IF EXISTS " + name).Error; err != nil {
				return fmt.Errorf("database: drop legacy index %s: %w", name, err)
			}
		}
	}
	return nil
}
