package database

// 迁移执行器回归（审核 H-14）：
//   - 两个方言的 up/down 必须成对、版本连续、简述一致；
//   - 全新 SQLite 库按序执行全部 up，再全部 down 能回到空库；
//   - 迁移建出的结构与 AutoMigrate 建出的结构一致（防止模型变更只改代码不补迁移）；
//   - 已有库首次接入时基线化，不重跑 0001；
//   - 设置 TEST_POSTGRES_DSN 时在 Postgres 上跑同一套 up/down（CI 用）。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"apidoc/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openSQLite 打开临时文件 SQLite（不用 :memory:，避免连接池各连接看到不同的内存库）。
func openSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "migrate.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("打开 SQLite: %v", err)
	}
	closeDB(t, db)
	return db
}

func closeDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
}

// schemaTables 返回用户表集合（排除 sqlite 内部表）。
func schemaTables(t *testing.T, db *gorm.DB) map[string]bool {
	t.Helper()
	var names []string
	if err := db.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&names).Error; err != nil {
		t.Fatalf("读取表清单: %v", err)
	}
	out := make(map[string]bool, len(names))
	for _, n := range names {
		out[n] = true
	}
	return out
}

// schemaColumns 返回某表的列名集合。
func schemaColumns(t *testing.T, db *gorm.DB, table string) map[string]bool {
	t.Helper()
	cols, err := db.Migrator().ColumnTypes(table)
	if err != nil {
		t.Fatalf("读取 %s 列: %v", table, err)
	}
	out := make(map[string]bool, len(cols))
	for _, c := range cols {
		out[c.Name()] = true
	}
	return out
}

// schemaIndexes 返回显式索引名集合（不包含 sqlite_autoindex_*）。
func schemaIndexes(t *testing.T, db *gorm.DB) map[string]bool {
	t.Helper()
	var names []string
	if err := db.Raw("SELECT name FROM sqlite_master WHERE type='index' AND sql IS NOT NULL").Scan(&names).Error; err != nil {
		t.Fatalf("读取索引清单: %v", err)
	}
	out := make(map[string]bool, len(names))
	for _, n := range names {
		out[n] = true
	}
	return out
}

// TestMigrationPairs 两个方言的版本必须连续、up/down 成对、简述一致。
func TestMigrationPairs(t *testing.T) {
	t.Parallel()
	if err := validateMigrationSets(); err != nil {
		t.Fatalf("迁移集合校验失败: %v", err)
	}
	for _, dialect := range []string{"sqlite", "postgres"} {
		all, err := loadMigrations(dialect)
		if err != nil {
			t.Fatalf("loadMigrations(%s): %v", dialect, err)
		}
		versions := sortedVersions(all)
		if len(versions) == 0 {
			t.Fatalf("%s 没有迁移", dialect)
		}
		for i, v := range versions {
			if v != i+1 {
				t.Fatalf("%s 版本不连续：第 %d 个是 %d", dialect, i+1, v)
			}
		}
	}
}

// TestSQLiteMigrationsApply 全新库按序执行全部 up，并记录版本。
func TestSQLiteMigrationsApply(t *testing.T) {
	t.Parallel()
	db := openSQLite(t)
	if err := MigrateRelease(db); err != nil {
		t.Fatalf("MigrateRelease: %v", err)
	}
	all, err := loadMigrations("sqlite")
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	var count int64
	if err := db.Table(schemaMigrationsTable).Count(&count).Error; err != nil {
		t.Fatalf("统计 schema_migrations: %v", err)
	}
	if int(count) != len(all) {
		t.Fatalf("已应用版本数 = %d，want %d", count, len(all))
	}
	for _, table := range []string{"change_log", "sync_op", "user_token", "api_return_sample", "field_mapping"} {
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
		{"file", "hash"}, {"file", "size"}, {"file", "mime"},
	} {
		if !db.Migrator().HasColumn(col.table, col.column) {
			t.Errorf("迁移完成后缺少列 %s.%s", col.table, col.column)
		}
	}
}

// TestSQLiteMigrateReleaseIdempotent 重复启动不应重复执行迁移。
func TestSQLiteMigrateReleaseIdempotent(t *testing.T) {
	t.Parallel()
	db := openSQLite(t)
	if err := MigrateRelease(db); err != nil {
		t.Fatalf("first MigrateRelease: %v", err)
	}
	if err := MigrateRelease(db); err != nil {
		t.Fatalf("second MigrateRelease: %v", err)
	}
	all, err := loadMigrations("sqlite")
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	var count int64
	if err := db.Table(schemaMigrationsTable).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if int(count) != len(all) {
		t.Fatalf("重复启动后版本数 = %d，want %d", count, len(all))
	}
}

// TestSQLiteMigrationsRollback 全部 up 后全部 down，业务表应清空、记账表为空。
func TestSQLiteMigrationsRollback(t *testing.T) {
	t.Parallel()
	db := openSQLite(t)
	if err := MigrateRelease(db); err != nil {
		t.Fatalf("MigrateRelease: %v", err)
	}
	if err := MigrateDown(db, 0); err != nil {
		t.Fatalf("MigrateDown: %v", err)
	}
	for _, table := range []string{"user_info", "api", "project", "change_log", "variable", "share_link", "api_return_sample", "file"} {
		if db.Migrator().HasTable(table) {
			t.Errorf("全部回滚后不应残留表 %s", table)
		}
	}
	var count int64
	if err := db.Table(schemaMigrationsTable).Count(&count).Error; err != nil {
		t.Fatalf("统计 schema_migrations: %v", err)
	}
	if count != 0 {
		t.Fatalf("全部回滚后 schema_migrations 应为空，got %d", count)
	}
}

// TestLegacyBaseline 已有库（AutoMigrate 建、无 schema_migrations）首次接入：
// 不重跑 0001、不报错，全部当前版本记为已应用。
func TestLegacyBaseline(t *testing.T) {
	t.Parallel()
	db := openSQLite(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := MigrateRelease(db); err != nil {
		t.Fatalf("MigrateRelease(legacy): %v", err)
	}
	all, err := loadMigrations("sqlite")
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	var count int64
	if err := db.Table(schemaMigrationsTable).Count(&count).Error; err != nil {
		t.Fatalf("统计 schema_migrations: %v", err)
	}
	if int(count) != len(all) {
		t.Fatalf("基线化版本数 = %d，want %d", count, len(all))
	}
	if !db.Migrator().HasIndex(&model.UserInfo{}, "uk_user_name_active") {
		t.Fatal("基线化后应存在 uk_user_name_active 索引")
	}
}

// TestLegacyBaselineRejectsDuplicateUsers 历史同名校验：基线化前给出可操作的错误。
func TestLegacyBaselineRejectsDuplicateUsers(t *testing.T) {
	t.Parallel()
	db := openSQLite(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := db.Exec("DROP INDEX IF EXISTS uk_user_name_active").Error; err != nil {
		t.Fatalf("drop index: %v", err)
	}
	for _, email := range []string{"a@e.com", "b@e.com"} {
		if err := db.Create(&model.UserInfo{Name: "alice", Email: email, Pwd: "x"}).Error; err != nil {
			t.Fatalf("create alice(%s): %v", email, err)
		}
	}
	err := MigrateRelease(db)
	if err == nil || !strings.Contains(err.Error(), "同名活跃用户") {
		t.Fatalf("存在同名历史账号时应拒绝启动并给出提示，got %v", err)
	}
}

// TestMigrationsMatchModels 迁移脚本建出的结构必须与 AutoMigrate 一致。
// 这是「模型加了字段但忘记补迁移」的最后一道守卫。
func TestMigrationsMatchModels(t *testing.T) {
	t.Parallel()
	mig := openSQLite(t)
	if err := MigrateRelease(mig); err != nil {
		t.Fatalf("MigrateRelease: %v", err)
	}
	auto := openSQLite(t)
	if err := AutoMigrate(auto); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	migTables, autoTables := schemaTables(t, mig), schemaTables(t, auto)
	delete(migTables, schemaMigrationsTable)
	delete(autoTables, schemaMigrationsTable)
	for table := range autoTables {
		if !migTables[table] {
			t.Errorf("迁移缺少表 %s", table)
		}
	}
	for table := range migTables {
		if !autoTables[table] {
			t.Errorf("迁移多出表 %s", table)
		}
	}

	for table := range autoTables {
		if !migTables[table] {
			continue
		}
		mc, ac := schemaColumns(t, mig, table), schemaColumns(t, auto, table)
		for c := range ac {
			if !mc[c] {
				t.Errorf("迁移缺少列 %s.%s", table, c)
			}
		}
		for c := range mc {
			if !ac[c] {
				t.Errorf("迁移多出列 %s.%s", table, c)
			}
		}
	}

	mi, ai := schemaIndexes(t, mig), schemaIndexes(t, auto)
	for name := range ai {
		if !mi[name] {
			t.Errorf("迁移缺少索引 %s", name)
		}
	}
	for name := range mi {
		if !ai[name] {
			t.Errorf("迁移多出索引 %s", name)
		}
	}
}

// TestUserNameEmailPartialUnique 锁定 H-15：name/email 各自唯一（空 email 不参与），
// 软删除后 name/email 可复用。
func TestUserNameEmailPartialUnique(t *testing.T) {
	t.Parallel()
	db := openSQLite(t)
	if err := MigrateRelease(db); err != nil {
		t.Fatalf("MigrateRelease: %v", err)
	}

	alice := &model.UserInfo{Name: "alice", Email: "a@e.com", Pwd: "x"}
	if err := db.Create(alice).Error; err != nil {
		t.Fatalf("建首个用户: %v", err)
	}
	// 同名不同邮箱：必须被 name 唯一索引拒绝。
	if err := db.Create(&model.UserInfo{Name: "alice", Email: "b@e.com", Pwd: "x"}).Error; err == nil {
		t.Fatal("同名不同邮箱应被拒绝（H-15）")
	}
	// 同邮箱不同名：必须被 email 唯一索引拒绝。
	if err := db.Create(&model.UserInfo{Name: "bob", Email: "a@e.com", Pwd: "x"}).Error; err == nil {
		t.Fatal("同邮箱不同名应被拒绝（H-15）")
	}

	// 软删后可复用 name/email。
	if err := db.Model(&model.UserInfo{}).Where("id = ?", alice.ID).Update("is_deleted", 1).Error; err != nil {
		t.Fatalf("软删: %v", err)
	}
	if err := db.Create(&model.UserInfo{Name: "alice", Email: "a@e.com", Pwd: "x"}).Error; err != nil {
		t.Fatalf("软删用户的 name/email 应可复用: %v", err)
	}

	// 空邮箱不参与唯一：多个空邮箱用户可共存。
	for _, name := range []string{"no-mail-1", "no-mail-2"} {
		if err := db.Create(&model.UserInfo{Name: name, Email: "", Pwd: "x"}).Error; err != nil {
			t.Fatalf("空邮箱用户 %s 不应被邮箱索引拒绝: %v", name, err)
		}
	}
}

// TestSplitSQL 覆盖注释、字符串、标识符与 dollar-quoted 块中的分号。
func TestSplitSQL(t *testing.T) {
	t.Parallel()
	script := "-- comment; not a stmt\n" +
		"CREATE TABLE a (x TEXT DEFAULT 'a;b'); /* block ; */\n" +
		"INSERT INTO a VALUES ('it''s;ok');\n" +
		"DO $$ BEGIN RAISE NOTICE 'x;y'; END $$;\n" +
		"SELECT 1;"
	stmts := splitSQL(script)
	if len(stmts) != 4 {
		t.Fatalf("splitSQL 得到 %d 条语句，want 4:\n%s", len(stmts), strings.Join(stmts, "\n---\n"))
	}
	if !strings.Contains(stmts[1], "it''s;ok") {
		t.Fatalf("字符串内的分号不能被拆开: %q", stmts[1])
	}
	if !strings.Contains(stmts[2], "$$") {
		t.Fatalf("dollar-quoted 块应保持完整: %q", stmts[2])
	}
}

// TestPostgresMigrationsApply 在真实 PostgreSQL 上跑全部 up/down（CI 设置 TEST_POSTGRES_DSN）。
func TestPostgresMigrationsApply(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("未设置 TEST_POSTGRES_DSN，跳过 PostgreSQL 迁移测试")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("打开 PostgreSQL: %v", err)
	}
	closeDB(t, db)
	if os.Getenv("TEST_POSTGRES_RESET") == "1" {
		if err := db.Exec("DROP SCHEMA public CASCADE").Error; err != nil {
			t.Fatalf("reset schema: %v", err)
		}
		if err := db.Exec("CREATE SCHEMA public").Error; err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	if err := MigrateRelease(db); err != nil {
		t.Fatalf("MigrateRelease(postgres): %v", err)
	}
	if !db.Migrator().HasTable("user_info") || !db.Migrator().HasColumn("file", "hash") {
		t.Fatal("PostgreSQL 迁移后缺少表/列")
	}
	if err := MigrateDown(db, 0); err != nil {
		t.Fatalf("MigrateDown(postgres): %v", err)
	}
	if db.Migrator().HasTable("user_info") {
		t.Fatal("PostgreSQL 全部回滚后不应残留 user_info")
	}
}
