package database

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"apidoc/internal/model"
)

// migrationsDir 迁移脚本目录（相对本包：server/internal/database → server/migrations）。
const migrationsDir = "../../migrations"

// upFilePattern 匹配 0001_init.up.sql，捕获版本号与简述。
var upFilePattern = regexp.MustCompile(`^(\d{4})_([a-z0-9_]+)\.up\.sql$`)

// upFiles 返回某方言目录下按版本号升序排列的 up 脚本。
func upFiles(t *testing.T, dialect string) []string {
	t.Helper()
	dir := filepath.Join(migrationsDir, dialect)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取迁移目录 %s: %v", dir, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		t.Fatalf("%s 下没有 up 迁移脚本", dir)
	}
	return names
}

// TestSQLiteMigrationsApply sqlite 的 up 脚本必须能按版本号顺序完整执行。
// 这条守卫针对的是一类真实事故：迁移脚本只被写出来、从未被执行过，
// 于是 0010 的 `BIGINT PRIMARY KEY AUTOINCREMENT`（SQLite 不允许）与
// 0011 的 `ALTER TABLE user_token`（表尚未创建）长期无人发现。
func TestSQLiteMigrationsApply(t *testing.T) {
	t.Parallel()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("打开内存库: %v", err)
	}
	for _, name := range upFiles(t, "sqlite") {
		raw, err := os.ReadFile(filepath.Join(migrationsDir, "sqlite", name))
		if err != nil {
			t.Fatalf("读取 %s: %v", name, err)
		}
		if err := db.Exec(string(raw)).Error; err != nil {
			t.Fatalf("%s 执行失败: %v", name, err)
		}
	}

	// 同步地基的表与列必须真的落地（AutoMigrate 之外的手工迁移路径同样可用）
	for _, table := range []string{"change_log", "sync_op", "user_token"} {
		if !db.Migrator().HasTable(table) {
			t.Errorf("迁移完成后缺少表 %s", table)
		}
	}
	for _, col := range []struct{ table, column string }{
		{"api", "ext_uid"}, {"api", "last_change_id"},
		{"group", "ext_uid"}, {"doc", "ext_uid"},
		{"project", "client_write_enabled"},
		{"user_token", "token_cipher"},
		{"variable", "ext_uid"}, {"variable_env", "ext_uid"},
	} {
		if !db.Migrator().HasColumn(col.table, col.column) {
			t.Errorf("迁移完成后缺少列 %s.%s", col.table, col.column)
		}
	}
}

// TestUserNameEmailPartialUnique 锁定 B-18：user_info 的 name/email 唯一索引必须带
// `where is_deleted = 0`。否则应用层 UserTaken（走 active() 排除软删）判定「名字可用」，
// 插入却撞 DB 约束，两边结论互相矛盾。
func TestUserNameEmailPartialUnique(t *testing.T) {
	t.Parallel()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("打开内存库: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	old := &model.UserInfo{Name: "alice", Email: "a@e.com", Pwd: "x"}
	if err := db.Create(old).Error; err != nil {
		t.Fatalf("建首个用户: %v", err)
	}
	// 软删除首个用户
	if err := db.Model(&model.UserInfo{}).Where("id = ?", old.ID).Update("is_deleted", 1).Error; err != nil {
		t.Fatalf("软删: %v", err)
	}

	// 软删后同 name/email 必须可以再用
	reuse := &model.UserInfo{Name: "alice", Email: "a@e.com", Pwd: "x"}
	if err := db.Create(reuse).Error; err != nil {
		t.Fatalf("软删用户的 name/email 应可复用，got: %v", err)
	}

	// 但两个「活跃」用户的 name/email 仍必须冲突
	dup := &model.UserInfo{Name: "alice", Email: "a@e.com", Pwd: "x"}
	if err := db.Create(dup).Error; err == nil {
		t.Error("活跃用户的 name/email 重复应被唯一索引拒绝")
	}
}

// TestMigrationVersionParity 两个方言的版本号必须一一对应。
// B-04 的根因就是 postgres 缺了 0010 而后续文件照旧从 0010 开始编号，
// 表现为「文件齐全、版本号连续」，但实际少了一份迁移、且 0011 的 ALTER 指向不存在的表。
func TestMigrationVersionParity(t *testing.T) {
	t.Parallel()
	versions := map[string]map[string]string{}
	for _, dialect := range []string{"sqlite", "postgres"} {
		seen := map[string]string{}
		for _, name := range upFiles(t, dialect) {
			m := upFilePattern.FindStringSubmatch(name)
			if m == nil {
				t.Errorf("%s: 文件名不符合 <4位版本号>_<简述>.up.sql：%s", dialect, name)
				continue
			}
			if dup, ok := seen[m[1]]; ok {
				t.Errorf("%s: 版本号 %s 重复（%s 与 %s）", dialect, m[1], dup, name)
			}
			seen[m[1]] = m[2]
		}
		versions[dialect] = seen
	}

	for version, sqName := range versions["sqlite"] {
		pgName, ok := versions["postgres"][version]
		if !ok {
			t.Errorf("版本 %s 只在 sqlite 存在（%s），postgres 缺该迁移", version, sqName)
			continue
		}
		if pgName != sqName {
			t.Errorf("版本 %s 两方言简述不一致：sqlite=%s postgres=%s", version, sqName, pgName)
		}
	}
	for version, pgName := range versions["postgres"] {
		if _, ok := versions["sqlite"][version]; !ok {
			t.Errorf("版本 %s 只在 postgres 存在（%s），sqlite 缺该迁移", version, pgName)
		}
	}
}
