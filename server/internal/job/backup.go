// Package job 后台任务。
package job

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"apidoc/internal/config"
)

// StartBackup 在启用且驱动为 sqlite 时，按间隔把数据文件复制到备份目录。
// 随 ctx 结束而停止；返回等待函数，供关停流程等待本轮备份收尾（审核 H-12）。
// 未启用时返回空等待函数。
func StartBackup(ctx context.Context, cfg *config.Config) (wait func()) {
	if !cfg.Backup.Enabled || cfg.DB.Driver != "sqlite" {
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
				if err := backupOnce(cfg); err != nil {
					log.Printf("backup: %v", err)
				}
			}
		}
	}()
	return wg.Wait
}

// backupOnce 复制数据文件到带时间戳的备份。
// 先写临时文件再原子改名：进程中途退出时只会留下 .tmp，不会产生「看似完整」的截断备份（审核 H-13）。
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
	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	log.Printf("backup: wrote %s", dst)
	return nil
}
