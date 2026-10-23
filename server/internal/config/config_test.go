package config

// 审核 H-20/H-22：release 必须显式配置 JWT_SECRET 与 ADMIN_PASSWORD；
// debug 允许随机生成（仅首次创建超管时提示口令）。
// 审核 H-19：TRUSTED_PROXIES 必须是合法 IP/CIDR。

import (
	"path/filepath"
	"testing"
)

func loadWith(t *testing.T, env map[string]string) (*Config, error) {
	t.Helper()
	t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "missing.yaml"))
	for k, v := range env {
		t.Setenv(k, v)
	}
	return Load()
}

func TestLoadReleaseRequiresSecrets(t *testing.T) {
	_, err := loadWith(t, map[string]string{
		"GIN_MODE": "release", "JWT_SECRET": "", "ADMIN_PASSWORD": "",
	})
	if err == nil {
		t.Fatal("release 缺少 JWT_SECRET/ADMIN_PASSWORD 应拒绝启动")
	}
}

func TestLoadReleaseWithExplicitSecrets(t *testing.T) {
	cfg, err := loadWith(t, map[string]string{
		"GIN_MODE": "release", "JWT_SECRET": "explicit-jwt", "ADMIN_PASSWORD": "explicit-admin",
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.JWT.Secret != "explicit-jwt" || cfg.Admin.Password != "explicit-admin" {
		t.Fatalf("显式配置未生效: jwt=%q admin=%q", cfg.JWT.Secret, cfg.Admin.Password)
	}
	if cfg.Admin.PasswordGenerated {
		t.Fatal("显式 ADMIN_PASSWORD 不应标记为随机生成")
	}
}

func TestLoadDebugGeneratesSecrets(t *testing.T) {
	cfg, err := loadWith(t, map[string]string{
		"GIN_MODE": "debug", "JWT_SECRET": "", "ADMIN_PASSWORD": "",
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.JWT.Secret == "" || cfg.Admin.Password == "" {
		t.Fatal("debug 应随机生成 JWT 与超管口令")
	}
	if !cfg.Admin.PasswordGenerated {
		t.Fatal("debug 随机口令应标记 PasswordGenerated")
	}
}

func TestLoadRejectsInvalidTrustedProxy(t *testing.T) {
	_, err := loadWith(t, map[string]string{
		"GIN_MODE": "debug", "TRUSTED_PROXIES": "not-an-ip",
	})
	if err == nil {
		t.Fatal("非法 TRUSTED_PROXIES 应拒绝启动")
	}
}
