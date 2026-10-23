// Package config 负责加载后端运行配置。
// 配置来源（优先级从低到高）：内置默认值 < YAML 配置文件 < 环境变量。
// 默认读取 config/config.yaml（相对运行目录，可用 CONFIG_FILE 覆盖）；
// 通过 DB_DRIVER 在 SQLite（默认）与 PostgreSQL 之间切换，业务代码不感知方言。
package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"

	"apidoc/internal/pkg/fileutil"

	"gopkg.in/yaml.v3"
)

// defaultConfigFile 为默认配置文件路径。
const defaultConfigFile = "config/config.yaml"

// defaultJWTSecret 是内置默认 JWT 密钥占位符。它同时用于派生 PAT 的 AES 密钥，
// 一旦在生产沿用即可伪造任意身份（含超管）并解密全量 PAT，因此必须被显式配置替换。
const defaultJWTSecret = "change-me-in-prod-apidoc-jwt-secret"

// Config 是应用运行期配置。
type Config struct {
	// HTTP 监听端口，默认 "2027"。
	Port string `yaml:"port"`
	// RunMode 为 gin 运行模式：debug / release / test。
	RunMode string `yaml:"run_mode"`

	// DB 数据库相关配置。
	DB DatabaseConfig `yaml:"db"`
	// JWT 令牌配置。
	JWT JWTConfig `yaml:"jwt"`
	// Upload 文件上传配置。
	Upload UploadConfig `yaml:"upload"`
	// Security 安全策略（限流等）。
	Security SecurityConfig `yaml:"security"`
	// Backup 定时备份配置。
	Backup BackupConfig `yaml:"backup"`
	// Log 操作日志保留策略。
	Log LogConfig `yaml:"log"`
	// Admin 初始超管账号（种子）。
	Admin AdminConfig `yaml:"admin"`
}

// LogConfig 操作日志清理配置。
type LogConfig struct {
	// RetentionDays 保留天数，超期硬删除；默认 70。
	RetentionDays int `yaml:"retention_days"`
}

// JWTConfig 签名令牌配置。
type JWTConfig struct {
	// Secret 签名密钥。
	Secret string `yaml:"secret"`
	// ExpireHours 令牌有效期（小时）。
	ExpireHours int `yaml:"expire_hours"`
}

// UploadConfig 上传配置。
type UploadConfig struct {
	// Dir 本地存储目录。
	Dir string `yaml:"dir"`
	// MaxMB 单文件大小上限（MB）。
	MaxMB int64 `yaml:"max_mb"`
	// AllowExt 允许的扩展名白名单（小写、带点）；为空时按默认白名单处理。
	AllowExt []string `yaml:"allow_ext"`
	// SignKey 上传目录访问签名密钥；为空表示 /uploads 公开可读（默认，便于分享页内嵌图片）。
	SignKey string `yaml:"sign_key"`
}

// SecurityConfig 安全相关配置。
type SecurityConfig struct {
	// RateLimitPerMin 全局限流：每 IP 每分钟请求数（0 = 关闭）。
	RateLimitPerMin int `yaml:"rate_limit_per_min"`
	// AuthRateLimitPerMin 敏感接口（登录/注册）限流：每 IP 每分钟请求数（0 = 关闭）。
	AuthRateLimitPerMin int `yaml:"auth_rate_limit_per_min"`
}

// BackupConfig 定时备份配置。
type BackupConfig struct {
	// Enabled 是否启用定时备份。
	Enabled bool `yaml:"enabled"`
	// IntervalMinutes 备份间隔（分钟）。
	IntervalMinutes int `yaml:"interval_minutes"`
	// Dir 备份输出目录。
	Dir string `yaml:"dir"`
	// Retention 保留最近 N 份备份，超出的按时间从旧到新删除；<=0 用默认 7（审核 H-13）。
	Retention int `yaml:"retention"`
}

// AdminConfig 初始超级管理员。
type AdminConfig struct {
	Name     string `yaml:"name"`
	Password string `yaml:"password"`
}

// DatabaseConfig 描述数据库连接参数。
type DatabaseConfig struct {
	// Driver 取值 "sqlite"（默认）或 "postgres"。
	Driver string `yaml:"driver"`
	// DSN 数据源名称。sqlite 为文件路径，postgres 为连接串。
	DSN string `yaml:"dsn"`
}

// Load 加载配置，优先级从低到高：内置默认值 < YAML 配置文件 < 环境变量。
// 配置文件默认读取 config/config.yaml（可用 CONFIG_FILE 覆盖）；文件不存在时跳过。
// 返回的 Config 保证字段可直接使用；当 Driver 非法时返回错误。
func Load() (*Config, error) {
	cfg := defaults()

	if b, err := os.ReadFile(configFilePath()); err == nil {
		if err := yaml.Unmarshal(b, cfg); err != nil {
			return nil, fmt.Errorf("config: parse %s: %w", configFilePath(), err)
		}
	}

	applyEnvOverrides(cfg)

	switch cfg.DB.Driver {
	case "sqlite", "postgres":
	default:
		return nil, fmt.Errorf("config: unsupported DB_DRIVER %q (want sqlite|postgres)", cfg.DB.Driver)
	}
	if cfg.DB.DSN == "" {
		cfg.DB.DSN = driverDefaultDSN(cfg.DB.Driver)
	}
	if len(cfg.Upload.AllowExt) == 0 {
		cfg.Upload.AllowExt = fileutil.ParseAllowExt("")
	}
	// 未显式配置 JWT_SECRET 时随机生成：沿用内置默认密钥可被伪造任意身份并解密全量 PAT。
	if cfg.JWT.Secret == "" || cfg.JWT.Secret == defaultJWTSecret {
		if s, err := randomSecret(); err == nil {
			cfg.JWT.Secret = s
			log.Printf("config: JWT_SECRET 未配置或仍为内置默认值，已随机生成临时密钥（重启后失效，请显式设置 JWT_SECRET）")
		} else {
			return nil, fmt.Errorf("config: 生成临时 JWT 密钥失败: %w", err)
		}
	}
	return cfg, nil
}

// randomSecret 生成 32 字节随机密钥的 hex 串（用于 JWT 签名与 PAT 加密派生）。
func randomSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// defaults 返回内置默认值。
func defaults() *Config {
	return &Config{
		Port:    "2027",
		RunMode: "debug",
		DB:      DatabaseConfig{Driver: "sqlite"},
		JWT: JWTConfig{
			Secret:      defaultJWTSecret,
			ExpireHours: 24,
		},
		Upload: UploadConfig{
			Dir:   "data/uploads",
			MaxMB: 5,
		},
		Security: SecurityConfig{
			RateLimitPerMin:     300,
			AuthRateLimitPerMin: 10,
		},
		Log:    LogConfig{RetentionDays: 70},
		Backup: BackupConfig{Enabled: false, IntervalMinutes: 1440, Dir: "data/backups", Retention: 7},
		Admin:  AdminConfig{Name: "admin", Password: "admin123"},
	}
}

// configFilePath 返回配置文件路径（CONFIG_FILE 优先，默认 config/config.yaml）。
func configFilePath() string {
	return getEnv("CONFIG_FILE", defaultConfigFile)
}

// driverDefaultDSN 返回未配置 DSN 时各驱动的默认连接参数。
func driverDefaultDSN(driver string) string {
	switch driver {
	case "postgres":
		return "host=localhost user=postgres password=postgres dbname=apidoc port=5432 sslmode=disable TimeZone=UTC"
	default:
		return "data/apidoc.db"
	}
}

// applyEnvOverrides 用非空环境变量覆盖 cfg 中对应配置（环境变量优先级最高）。
func applyEnvOverrides(cfg *Config) {
	overrideStr := func(dst *string, key string) {
		if v := os.Getenv(key); v != "" {
			*dst = v
		}
	}
	overrideInt := func(dst *int, key string) {
		if v := os.Getenv(key); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				*dst = n
			}
		}
	}

	overrideStr(&cfg.Port, "PORT")
	overrideStr(&cfg.RunMode, "GIN_MODE")
	overrideStr(&cfg.DB.Driver, "DB_DRIVER")
	overrideStr(&cfg.DB.DSN, "DB_DSN")
	overrideStr(&cfg.JWT.Secret, "JWT_SECRET")
	overrideInt(&cfg.JWT.ExpireHours, "JWT_EXPIRE_HOURS")
	overrideStr(&cfg.Upload.Dir, "UPLOAD_DIR")
	if v := os.Getenv("UPLOAD_MAX_MB"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.Upload.MaxMB = n
		}
	}
	if v := os.Getenv("UPLOAD_ALLOW_EXT"); v != "" {
		cfg.Upload.AllowExt = fileutil.ParseAllowExt(v)
	}
	overrideStr(&cfg.Upload.SignKey, "UPLOAD_SIGN_KEY")
	overrideInt(&cfg.Security.RateLimitPerMin, "RATE_LIMIT_PER_MIN")
	overrideInt(&cfg.Security.AuthRateLimitPerMin, "AUTH_RATE_LIMIT_PER_MIN")
	overrideInt(&cfg.Log.RetentionDays, "LOG_RETENTION_DAYS")
	if v := os.Getenv("BACKUP_ENABLED"); v != "" {
		cfg.Backup.Enabled = v == "true"
	}
	overrideInt(&cfg.Backup.IntervalMinutes, "BACKUP_INTERVAL_MIN")
	overrideStr(&cfg.Backup.Dir, "BACKUP_DIR")
	overrideInt(&cfg.Backup.Retention, "BACKUP_RETENTION")
	overrideStr(&cfg.Admin.Name, "ADMIN_NAME")
	overrideStr(&cfg.Admin.Password, "ADMIN_PASSWORD")
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
