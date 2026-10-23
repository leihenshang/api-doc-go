package main

// 一次性生成器：以 GORM AutoMigrate 结果为唯一事实来源，导出 SQLite 生产迁移脚本。
// 用法（在 server/ 目录）：go run ./data/genmigrate → 写 migrations/sqlite/0001_init.{up,down}.sql

import (
	"fmt"
	"os"
	"strings"

	"apidoc/internal/database"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type master struct {
	Type string
	Name string
	SQL  string
}

func main() {
	dbPath := "data/_gen.migrate.db"
	_ = os.Remove(dbPath)
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		panic(err)
	}
	// 生成器只按模型导出初始结构，不执行迁移（否则会把 schema_migrations 一起导出）。
	if err := database.AutoMigrate(db); err != nil {
		panic(err)
	}

	var items []master
	if err := db.Table("sqlite_master").Select("type, name, sql").
		Where("sql IS NOT NULL AND name NOT LIKE 'sqlite_%'").
		Order("CASE type WHEN 'table' THEN 0 ELSE 1 END, name").Scan(&items).Error; err != nil {
		panic(err)
	}

	var up strings.Builder
	up.WriteString("-- 0001_init：api-doc 初始结构（SQLite）。\n")
	up.WriteString("-- 由 data/genmigrate 从 AutoMigrate 导出，勿手改；模型变更后重新生成。\n")
	var names []string
	for _, it := range items {
		up.WriteString(strings.TrimSpace(it.SQL) + ";\n")
		if it.Type == "table" {
			names = append(names, it.Name)
		}
	}
	if err := os.MkdirAll("migrations/sqlite", 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile("migrations/sqlite/0001_init.up.sql", []byte(up.String()), 0o644); err != nil {
		panic(err)
	}

	var down strings.Builder
	down.WriteString("-- 回滚：删除全部业务表（索引随表删除）。\n")
	for i := len(names) - 1; i >= 0; i-- {
		fmt.Fprintf(&down, "DROP TABLE IF EXISTS `%s`;\n", names[i])
	}
	if err := os.WriteFile("migrations/sqlite/0001_init.down.sql", []byte(down.String()), 0o644); err != nil {
		panic(err)
	}
	_ = os.Remove(dbPath)
	fmt.Printf("generated %d statements, %d tables\n", len(items), len(names))
}
