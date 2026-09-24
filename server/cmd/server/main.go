package main

import (
	"context"
	"log"
	"os"
	"time"

	"apidoc/internal/config"
	"apidoc/internal/database"
	"apidoc/internal/job"
	"apidoc/internal/repository"
	"apidoc/internal/router"
	"apidoc/internal/service"
)

// main 是后端 HTTP 服务入口：加载配置 → 连接数据库 → 迁移 → 后台任务 → 装配路由 → 启动监听。
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if err := os.MkdirAll(cfg.Upload.Dir, 0o755); err != nil {
		log.Fatalf("create upload dir: %v", err)
	}

	db, err := database.Open(cfg)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		log.Fatalf("migrate database: %v", err)
	}

	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	// 默认超管种子：仅当 ADMIN_NAME 不存在时创建，不重置已有密码。
	svc := service.New(repository.New(db, cfg), cfg)
	created, err := svc.EnsureAdmin(ctx, cfg.Admin.Name, cfg.Admin.Password)
	if err != nil {
		log.Fatalf("seed admin: %v", err)
	}
	if created {
		log.Printf("default super admin created: name=%s password=%s （请尽快修改）", cfg.Admin.Name, cfg.Admin.Password)
	}

	job.StartBackup(ctx, cfg)
	job.StartCleanup(ctx, db, cfg.Log.RetentionDays, time.Hour)

	engine := router.Setup(db, cfg)

	addr := ":" + cfg.Port
	log.Printf("server listening on %s (driver=%s)", addr, cfg.DB.Driver)
	if err := engine.Run(addr); err != nil {
		log.Fatalf("server run: %v", err)
	}
}
