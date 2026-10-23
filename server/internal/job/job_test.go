package job

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"apidoc/internal/config"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openTestDB 打开一个临时 SQLite 库（与生产同驱动）。
func openTestDB(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

// TestBackupOnceWritesConsistentSnapshot 校验备份用 VACUUM INTO 生成一致性快照（审核 H-13）：
// 备份可直接打开、数据完整，不残留 .tmp，且权限收口为 0600。
func TestBackupOnceWritesConsistentSnapshot(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	cfg.DB.Driver = "sqlite"
	cfg.DB.DSN = filepath.Join(dir, "apidoc.db")
	cfg.Backup.Dir = filepath.Join(dir, "backups")

	db := openTestDB(t, cfg.DB.DSN)
	if err := db.Exec("CREATE TABLE t (v TEXT)").Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := db.Exec("INSERT INTO t VALUES ('payload')").Error; err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := backupOnce(db, cfg); err != nil {
		t.Fatalf("backupOnce: %v", err)
	}

	names, err := backupNames(cfg.Backup.Dir)
	if err != nil {
		t.Fatalf("backupNames: %v", err)
	}
	if len(names) != 1 {
		t.Fatalf("want 1 backup file, got %v", names)
	}
	full := filepath.Join(cfg.Backup.Dir, names[0])
	st, err := os.Stat(full)
	if err != nil {
		t.Fatalf("stat backup: %v", err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("backup mode = %v, want 0600", st.Mode().Perm())
	}
	// 快照必须可独立打开且数据完整（直接复制主文件会漏掉 WAL 中已提交的事务）。
	bdb := openTestDB(t, full)
	var got string
	if err := bdb.Raw("SELECT v FROM t").Scan(&got).Error; err != nil || got != "payload" {
		t.Fatalf("backup content = %q (err=%v), want payload", got, err)
	}
	entries, _ := os.ReadDir(cfg.Backup.Dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("残留临时文件 %q", e.Name())
		}
	}
}

// TestBackupRetention 保留最近 N 份，旧备份按时间清理（审核 H-13）。
func TestBackupRetention(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	cfg.DB.Driver = "sqlite"
	cfg.DB.DSN = filepath.Join(dir, "apidoc.db")
	cfg.Backup.Dir = filepath.Join(dir, "backups")
	cfg.Backup.Retention = 2

	db := openTestDB(t, cfg.DB.DSN)
	if err := db.Exec("CREATE TABLE t (v TEXT)").Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := backupOnce(db, cfg); err != nil {
			t.Fatalf("backupOnce #%d: %v", i, err)
		}
		time.Sleep(2 * time.Millisecond) // 保证时间戳不同，不会互相覆盖
	}
	names, err := backupNames(cfg.Backup.Dir)
	if err != nil {
		t.Fatalf("backupNames: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("retention=2 应只剩 2 份，got %v", names)
	}
}

// TestStartBackupDisabled 未启用备份时返回的等待函数应立即返回且不阻塞。
func TestStartBackupDisabled(t *testing.T) {
	wait := StartBackup(context.Background(), nil, &config.Config{})
	done := make(chan struct{})
	go func() { wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("wait blocked when backup disabled")
	}
}

// TestStartBackupStopsOnCancel 校验 ctx 取消后备份任务退出，wait 可等到收尾。
func TestStartBackupStopsOnCancel(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	cfg.DB.Driver = "sqlite"
	cfg.DB.DSN = filepath.Join(dir, "apidoc.db")
	cfg.Backup.Enabled = true
	cfg.Backup.IntervalMinutes = 60
	cfg.Backup.Dir = filepath.Join(dir, "backups")

	db := openTestDB(t, cfg.DB.DSN)
	ctx, cancel := context.WithCancel(context.Background())
	wait := StartBackup(ctx, db, cfg)
	cancel()
	done := make(chan struct{})
	go func() { wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("backup did not stop after cancel")
	}
}

// TestStartCleanupDisabled 保留期为 0 时不启动任务，等待函数立即返回。
func TestStartCleanupDisabled(t *testing.T) {
	wait := StartCleanup(context.Background(), nil, 0, time.Hour)
	done := make(chan struct{})
	go func() { wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("wait blocked when cleanup disabled")
	}
}
