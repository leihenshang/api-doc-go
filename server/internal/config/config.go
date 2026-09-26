// Package config 负责加载后端运行配置，来源优先级：环境变量 > 默认值。
// 通过 DB_DRIVER 在 SQLite（默认）与 PostgreSQL 之间切换，业务代码不感知方言。
package config

import (
	"fmt"
	"os"
	"strconv"

	"apidoc/internal/pkg/fileutil"
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
	// Security 安全策略（限流等）。
	Security SecurityConfig
	// Backup 定时备份配置。
	Backup BackupConfig
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
	// MaxMB 单文件大小上限（MB）。
	MaxMB int64
	// AllowExt 允许的扩展名白名单（小写、带点）。
	AllowExt []string
	// SignKey 上传目录访问签名密钥；为空表示 /uploads 公开可读（默认，便于分享页内嵌图片）。
	SignKey string
}

// SecurityConfig 安全相关配置。
type SecurityConfig struct {
	// RateLimitPerMin 全局限流：每 IP 每分钟请求数（0 = 关闭）。
	RateLimitPerMin int
	// AuthRateLimitPerMin 敏感接口（登录/注册）限流：每 IP 每分钟请求数（0 = 关闭）。
	AuthRateLimitPerMin int
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
	cfg.Upload = UploadConfig{
		Dir:      getEnv("UPLOAD_DIR", "data/uploads"),
		MaxMB:    int64(getEnvInt("UPLOAD_MAX_MB", 5)),
		AllowExt: fileutil.ParseAllowExt(getEnv("UPLOAD_ALLOW_EXT", "")),
		SignKey:  getEnv("UPLOAD_SIGN_KEY", ""),
	}
	cfg.Security = SecurityConfig{
		RateLimitPerMin:     getEnvInt("RATE_LIMIT_PER_MIN", 300),
		AuthRateLimitPerMin: getEnvInt("AUTH_RATE_LIMIT_PER_MIN", 10),
	}
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
