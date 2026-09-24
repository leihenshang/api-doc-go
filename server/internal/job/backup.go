// Package job 后台任务。
package job

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"apidoc/internal/config"
)

// StartBackup 在启用且驱动为 sqlite 时，按间隔把数据文件复制到备份目录。
// 随 ctx 结束而停止；未启用则直接返回。
func StartBackup(ctx context.Context, cfg *config.Config) {
	if !cfg.Backup.Enabled || cfg.DB.Driver != "sqlite" {
		return
	}
	interval := time.Duration(cfg.Backup.IntervalMinutes) * time.Minute
	if interval <= 0 {
		interval = time.Hour
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := backupOnce(cfg); err != nil {
					log.Printf("backup: %v", err)
				}
			}
		}
	}()
}

// backupOnce 复制数据文件到带时间戳的备份。
func backupOnce(cfg *config.Config) error {
	if err := os.MkdirAll(cfg.Backup.Dir, 0o755); err != nil {
		return err
	}
	src, err := os.Open(cfg.DB.DSN)
	if err != nil {
		return err
	}
	defer src.Close()
	dst := filepath.Join(cfg.Backup.Dir, time.Now().Format("20060102-150405")+"-apidoc.db")
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, src); err != nil {
		return err
	}
	log.Printf("backup: wrote %s", dst)
	return nil
}
