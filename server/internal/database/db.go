// Package database 负责按配置打开 GORM 连接（SQLite 默认 / PostgreSQL 可选）。
// 方言差异只体现在此文件，业务层不感知。
package database

import (
	"fmt"
	"os"
	"path/filepath"

	"apidoc/internal/config"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open 依据 cfg.DB 建立连接并返回 *gorm.DB。
// 当驱动不支持或连接失败时返回包装后的错误。
func Open(cfg *config.Config) (*gorm.DB, error) {
	dialector, err := dialectorFor(cfg)
	if err != nil {
		return nil, err
	}

	db, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		return nil, fmt.Errorf("database: open: %w", err)
	}
	return db, nil
}

// dialectorFor 根据配置返回对应 GORM 方言实现。
func dialectorFor(cfg *config.Config) (gorm.Dialector, error) {
	switch cfg.DB.Driver {
	case "sqlite":
		// SQLite 文件所在目录若不存在需先创建，否则连接失败。
		if dir := filepath.Dir(cfg.DB.DSN); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("database: create sqlite dir: %w", err)
			}
		}
		return sqlite.Open(cfg.DB.DSN), nil
	case "postgres":
		return postgres.Open(cfg.DB.DSN), nil
	default:
		return nil, fmt.Errorf("database: unsupported driver %q", cfg.DB.Driver)
	}
}
