// seed 确保超级管理员存在（幂等）。
//
// 默认行为：仅当 ADMIN_NAME 不存在时创建，不修改已有账号口令；
// 需要把已有超管口令重置为 ADMIN_PASSWORD 时必须显式加 -force（审核 H-21）。
// 读取环境变量 ADMIN_NAME（默认 admin）与 ADMIN_PASSWORD（release 必须显式配置）。
package main

import (
	"context"
	"flag"
	"log"

	"apidoc/internal/config"
	"apidoc/internal/database"
	"apidoc/internal/repository"
	"apidoc/internal/service"
)

func main() {
	force := flag.Bool("force", false, "重置已存在超管的口令（危险，仅显式指定时执行）")
	flag.Parse()

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
	ctx := context.Background()

	if *force {
		u, err := svc.ResetAdminPassword(ctx, cfg.Admin.Name, cfg.Admin.Password)
		if err != nil {
			log.Fatalf("reset admin: %v", err)
		}
		log.Printf("admin password reset: id=%d name=%s", u.ID, u.Name)
		return
	}

	created, err := svc.EnsureAdmin(ctx, cfg.Admin.Name, cfg.Admin.Password)
	if err != nil {
		log.Fatalf("ensure admin: %v", err)
	}
	if !created {
		log.Printf("admin already exists: name=%s（未修改口令；如需重置请加 -force）", cfg.Admin.Name)
		return
	}
	if cfg.Admin.PasswordGenerated {
		log.Printf("admin created: name=%s password=%s（随机生成，仅本次显示，请立即修改）", cfg.Admin.Name, cfg.Admin.Password)
		return
	}
	log.Printf("admin created: name=%s（请使用配置的 ADMIN_PASSWORD 登录）", cfg.Admin.Name)
}
