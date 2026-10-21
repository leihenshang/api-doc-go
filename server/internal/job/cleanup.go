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

// cleanup 物理删除超期 operation_log；并清掉软删超期的变量/环境/响应示例（审核 H-10）。
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
	purgeSoftDeleted(db, cutoff)
}

// purgeSoftDeleted 物理清理软删超过保留期的实体（审核 H-10：变量/环境/响应示例改软删后，
// 由本任务负责最终物理回收，避免 tombstone 永久堆积）。
func purgeSoftDeleted(db *gorm.DB, cutoff time.Time) {
	repo := repository.New(db, nil)
	ctx := context.Background()
	if n, err := repo.PurgeDeletedVariables(ctx, cutoff); err != nil {
		log.Printf("cleanup: purge variables: %v", err)
	} else if n > 0 {
		log.Printf("cleanup: purged %d soft-deleted variables", n)
	}
	if n, err := repo.PurgeDeletedVariableEnvs(ctx, cutoff); err != nil {
		log.Printf("cleanup: purge variable envs: %v", err)
	} else if n > 0 {
		log.Printf("cleanup: purged %d soft-deleted variable envs", n)
	}
	if n, err := repo.PurgeDeletedApiReturnSamples(ctx, cutoff); err != nil {
		log.Printf("cleanup: purge api samples: %v", err)
	} else if n > 0 {
		log.Printf("cleanup: purged %d soft-deleted api samples", n)
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
