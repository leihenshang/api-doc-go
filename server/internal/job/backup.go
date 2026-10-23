// Package job 后台任务。
package job

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"apidoc/internal/config"

	"gorm.io/gorm"
)

// defaultBackupRetention 未配置保留份数时的默认值（审核 H-13）。
const defaultBackupRetention = 7

// StartBackup 在启用且驱动为 sqlite 时，按间隔用 VACUUM INTO 生成一致性备份（审核 H-13）。
// 随 ctx 结束而停止；返回等待函数，供关停流程等待本轮备份收尾（审核 H-12）。
// 未启用时返回空等待函数。
func StartBackup(ctx context.Context, db *gorm.DB, cfg *config.Config) (wait func()) {
	if !cfg.Backup.Enabled {
		return func() {}
	}
	if cfg.DB.Driver != "sqlite" {
		// 内置备份依赖 SQLite 的 VACUUM INTO；PostgreSQL 生产环境请用 pg_dump/托管快照。
		log.Printf("backup: 内置备份仅支持 SQLite（当前 driver=%s），请使用 pg_dump 或托管快照", cfg.DB.Driver)
		return func() {}
	}
	interval := time.Duration(cfg.Backup.IntervalMinutes) * time.Minute
	if interval <= 0 {
		interval = time.Hour
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := backupOnce(db, cfg); err != nil {
					log.Printf("backup: %v", err)
				}
			}
		}
	}()
	return wg.Wait
}

// backupOnce 用 VACUUM INTO 生成一致性备份（审核 H-13）。
//
// 为什么不能直接复制活动库文件：已提交事务可能还在 -wal 里，io.Copy 只拿主文件会漏
// 数据，写入过程中的页级状态也可能不一致；VACUUM INTO 由 SQLite 按读事务导出新库，
// 天然包含 WAL 中已提交的内容，且单文件、无旁路文件。
//
// 先写唯一临时文件再原子改名：进程中途退出只会留下 .tmp，不会留下「看似完整」的截断备份。
func backupOnce(db *gorm.DB, cfg *config.Config) error {
	if db == nil {
		return fmt.Errorf("backup: 数据库连接为空")
	}
	if err := os.MkdirAll(cfg.Backup.Dir, 0o755); err != nil {
		return err
	}
	// 纳秒精度避免同一秒内两次备份互相覆盖。
	dst := filepath.Join(cfg.Backup.Dir, time.Now().Format("20060102-150405.000000000")+"-apidoc.db")
	tmp := dst + ".tmp"
	if err := vacuumInto(db, tmp); err != nil {
		return err
	}
	// 备份含变量表（可能存令牌）等完整数据，权限收口到 0600。
	if err := os.Chmod(tmp, 0o600); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("backup: chmod: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("backup: rename: %w", err)
	}
	if err := pruneBackups(cfg); err != nil {
		// 轮转失败不影响本次备份已成功，仅记录。
		log.Printf("backup: prune: %v", err)
	}
	log.Printf("backup: wrote %s", dst)
	return nil
}

// vacuumInto 让 SQLite 导出一致性快照到 path；失败时清理半成品。
func vacuumInto(db *gorm.DB, path string) error {
	_ = os.Remove(path) // VACUUM INTO 要求目标文件不存在
	if err := db.Exec("VACUUM INTO ?", path).Error; err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("backup: vacuum into: %w", err)
	}
	return nil
}

// pruneBackups 只保留最近 retention 份成功备份（文件名时间戳可直接字典序排序）。
func pruneBackups(cfg *config.Config) error {
	names, err := backupNames(cfg.Backup.Dir)
	if err != nil {
		return err
	}
	retention := cfg.Backup.Retention
	if retention <= 0 {
		retention = defaultBackupRetention
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for i, name := range names {
		if i < retention {
			continue
		}
		if err := os.Remove(filepath.Join(cfg.Backup.Dir, name)); err != nil {
			return err
		}
	}
	return nil
}

// backupNames 列出备份目录里的正式备份文件名（忽略 .tmp 等半成品）。
func backupNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "-apidoc.db") {
			continue
		}
		names = append(names, e.Name())
	}
	return names, nil
}
