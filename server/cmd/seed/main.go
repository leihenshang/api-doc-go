package main

import (
	"context"
	"log"

	"apidoc/internal/config"
	"apidoc/internal/database"
	"apidoc/internal/repository"
	"apidoc/internal/service"
)

// seed 初始化/重置超级管理员账号（幂等）。
// 读取环境变量 ADMIN_NAME（默认 admin）与 ADMIN_PASSWORD（默认 admin123）。
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	db, err := database.Open(cfg)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	if err := database.MigrateRelease(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	svc := service.New(repository.New(db, cfg), cfg)
	u, err := svc.InitAdmin(context.Background(), cfg.Admin.Name, cfg.Admin.Password)
	if err != nil {
		log.Fatalf("init admin: %v", err)
	}
	log.Printf("admin ready: id=%d name=%s", u.ID, u.Name)
}
