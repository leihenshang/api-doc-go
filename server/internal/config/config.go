// Package config 负责加载后端运行配置，来源优先级：环境变量 > 默认值。
// 通过 DB_DRIVER 在 SQLite（默认）与 PostgreSQL 之间切换，业务代码不感知方言。
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config 是应用运行期配置。
type Config struct {
	// HTTP 监听端口，例如 "8080"。
	Port string
	// RunMode 为 gin 运行模式：debug / release / test。
	RunMode string

	// DB 数据库相关配置。
	DB DatabaseConfig
	// JWT 令牌配置。
	JWT JWTConfig
	// Upload 文件上传配置。
	Upload UploadConfig
	// Backup 定时备份配置。
	Backup BackupConfig
	// Encrypt 敏感字段存储加密配置。
	Encrypt EncryptConfig
	// Log 操作日志保留策略。
	Log LogConfig
	// Admin 初始超管账号（种子）。
	Admin AdminConfig
}

// LogConfig 操作日志清理配置。
type LogConfig struct {
	// RetentionDays 保留天数，超期硬删除；默认 70。
	RetentionDays int
}

// EncryptConfig 外部数据源口令等敏感字段的加密配置。
type EncryptConfig struct {
	// SecretKey AES-256-GCM 主密钥（环境变量 SECRET_ENCRYPT_KEY）。
	SecretKey string
}

// JWTConfig 签名令牌配置。
type JWTConfig struct {
	// Secret 签名密钥。
	Secret string
	// ExpireHours 令牌有效期（小时）。
	ExpireHours int
}

// UploadConfig 上传配置。
type UploadConfig struct {
	// Dir 本地存储目录。
	Dir string
}

// BackupConfig 定时备份配置。
type BackupConfig struct {
	// Enabled 是否启用定时备份。
	Enabled bool
	// IntervalMinutes 备份间隔（分钟）。
	IntervalMinutes int
	// Dir 备份输出目录。
	Dir string
}

// AdminConfig 初始超级管理员。
type AdminConfig struct {
	Name     string
	Password string
}

// DatabaseConfig 描述数据库连接参数。
type DatabaseConfig struct {
	// Driver 取值 "sqlite"（默认）或 "postgres"。
	Driver string
	// DSN 数据源名称。sqlite 为文件路径，postgres 为连接串。
	DSN string
}

// Load 从环境变量读取配置并补齐默认值。
// 返回的 Config 保证字段可直接使用；当 Driver 非法时返回错误。
func Load() (*Config, error) {
	cfg := &Config{
		Port:    getEnv("PORT", "8080"),
		RunMode: getEnv("GIN_MODE", "debug"),
		DB: DatabaseConfig{
			Driver: getEnv("DB_DRIVER", "sqlite"),
		},
	}

	switch cfg.DB.Driver {
	case "sqlite":
		cfg.DB.DSN = getEnv("DB_DSN", "data/apidoc.db")
	case "postgres":
		cfg.DB.DSN = getEnv("DB_DSN", "host=localhost user=postgres password=postgres dbname=apidoc port=5432 sslmode=disable TimeZone=UTC")
	default:
		return nil, fmt.Errorf("config: unsupported DB_DRIVER %q (want sqlite|postgres)", cfg.DB.Driver)
	}

	cfg.JWT = JWTConfig{
		Secret:      getEnv("JWT_SECRET", "change-me-in-prod-apidoc-jwt-secret"),
		ExpireHours: getEnvInt("JWT_EXPIRE_HOURS", 24),
	}
	cfg.Upload = UploadConfig{Dir: getEnv("UPLOAD_DIR", "data/uploads")}
	cfg.Encrypt = EncryptConfig{SecretKey: getEnv("SECRET_ENCRYPT_KEY", "apidoc-dev-encrypt-key-change-me")}
	cfg.Log = LogConfig{RetentionDays: getEnvInt("LOG_RETENTION_DAYS", 70)}
	cfg.Backup = BackupConfig{
		Enabled:         getEnv("BACKUP_ENABLED", "false") == "true",
		IntervalMinutes: getEnvInt("BACKUP_INTERVAL_MIN", 1440),
		Dir:             getEnv("BACKUP_DIR", "data/backups"),
	}
	cfg.Admin = AdminConfig{
		Name:     getEnv("ADMIN_NAME", "admin"),
		Password: getEnv("ADMIN_PASSWORD", "admin123"),
	}

	return cfg, nil
}

// getEnv 读取环境变量，为空时返回默认值 def。
func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// getEnvInt 读取整型环境变量，解析失败时返回默认值 def。
func getEnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
