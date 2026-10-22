package job

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"apidoc/internal/config"
)

// TestBackupOnceWritesAtomically 校验备份先写临时文件再原子改名：
// 正常情况下产出正式文件、内容一致，且不残留 .tmp（审核 H-13）。
func TestBackupOnceWritesAtomically(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "apidoc.db")
	if err := os.WriteFile(src, []byte("payload"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	cfg := &config.Config{}
	cfg.DB.Driver = "sqlite"
	cfg.DB.DSN = src
	cfg.Backup.Dir = filepath.Join(dir, "backups")

	if err := backupOnce(cfg); err != nil {
		t.Fatalf("backupOnce: %v", err)
	}
	entries, err := os.ReadDir(cfg.Backup.Dir)
	if err != nil {
		t.Fatalf("read backups: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 backup file, got %d", len(entries))
	}
	name := entries[0].Name()
	if strings.HasSuffix(name, ".tmp") || !strings.HasSuffix(name, "-apidoc.db") {
		t.Fatalf("unexpected backup name %q", name)
	}
	got, err := os.ReadFile(filepath.Join(cfg.Backup.Dir, name))
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(got) != "payload" {
		t.Fatalf("backup content = %q, want payload", got)
	}
}

// TestStartBackupDisabled 未启用备份时返回的等待函数应立即返回且不阻塞。
func TestStartBackupDisabled(t *testing.T) {
	wait := StartBackup(context.Background(), &config.Config{})
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

	ctx, cancel := context.WithCancel(context.Background())
	wait := StartBackup(ctx, cfg)
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
