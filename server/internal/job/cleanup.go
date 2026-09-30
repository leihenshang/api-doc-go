package job

import (
	"context"
	"log"
	"time"

	"apidoc/internal/repository"

	"gorm.io/gorm"
)

// StartCleanup 周期清理超期日志与变更流水（启动即执行一次）。
// retentionDays<=0 表示不清理；interval 为清理周期。随 ctx 结束停止。
func StartCleanup(ctx context.Context, db *gorm.DB, retentionDays int, interval time.Duration) {
	if retentionDays <= 0 {
		return
	}
	if interval <= 0 {
		interval = time.Hour
	}
	go func() {
		cleanup(db, retentionDays)
		cleanupChangeLog(db)
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				cleanup(db, retentionDays)
				cleanupChangeLog(db)
			}
		}
	}()
}

// cleanup 硬删除超期 operation_log（日志无软删除语义，直接 DELETE）。
func cleanup(db *gorm.DB, retentionDays int) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	res := db.Exec("DELETE FROM operation_log WHERE create_time < ?", cutoff)
	if res.Error != nil {
		log.Printf("cleanup: %v", res.Error)
		return
	}
	if res.RowsAffected > 0 {
		log.Printf("cleanup: removed %d operation logs older than %s", res.RowsAffected, cutoff.Format("2006-01-02"))
	}
}

// cleanupChangeLog 变更流水清理（S6）：保留 ≥90 天或 ≥10 万条（取大）。
func cleanupChangeLog(db *gorm.DB) {
	repo := repository.New(db, nil)
	n, err := repo.CleanupChangeLog(context.Background())
	if err != nil {
		log.Printf("cleanup: change_log: %v", err)
		return
	}
	if n > 0 {
		log.Printf("cleanup: removed %d change_log rows", n)
	}
}
