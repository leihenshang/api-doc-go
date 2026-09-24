package job

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"
)

// StartCleanup 周期清理超过保留期的操作日志（启动即执行一次）。
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
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				cleanup(db, retentionDays)
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
