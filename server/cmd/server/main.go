package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
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
	if err := run(); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// run 完成服务装配与生命周期管理：收到 SIGINT/SIGTERM 后优雅关闭 HTTP 服务，
// 等待后台任务收尾并关闭数据库连接池（审核 H-12）。
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := os.MkdirAll(cfg.Upload.Dir, 0o755); err != nil {
		return fmt.Errorf("create upload dir: %w", err)
	}

	db, err := database.Open(cfg)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("database pool: %w", err)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			log.Printf("close database: %v", err)
		}
	}()
	if err := database.Migrate(db); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	// 信号驱动：ctx 取消后 HTTP 进入优雅关闭，后台任务（备份/清理）随之停止。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 默认超管种子：仅当 ADMIN_NAME 不存在时创建，不重置已有密码。
	svc := service.New(repository.New(db, cfg), cfg)
	created, err := svc.EnsureAdmin(ctx, cfg.Admin.Name, cfg.Admin.Password)
	if err != nil {
		return fmt.Errorf("seed admin: %w", err)
	}
	if created {
		log.Printf("default super admin created: name=%s password=%s （请尽快修改）", cfg.Admin.Name, cfg.Admin.Password)
	}

	waitBackup := job.StartBackup(ctx, db, cfg)
	waitCleanup := job.StartCleanup(ctx, db, cfg.Log.RetentionDays, time.Hour)

	engine := router.Setup(db, cfg)
	addr := ":" + cfg.Port
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	log.Printf("server listening on %s (driver=%s)", addr, cfg.DB.Driver)
	serveErr := serve(ctx, newHTTPServer(addr, engine), ln)

	// 先停后台任务并等本轮收尾（避免备份文件写一半），再关闭连接池。
	stop()
	waitBackup()
	waitCleanup()

	return serveErr
}
