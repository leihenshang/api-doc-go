// Package password 口令哈希。使用 bcrypt（cost 默认 10），自带随机盐。
// golang.org/x/crypto 已随 postgres 驱动进入依赖图，无额外下载成本。
package password

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// Generate 返回口令的 bcrypt 散列。
func Generate(plain string) (string, error) {
	sum, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("password: hash: %w", err)
	}
	return string(sum), nil
}

// Verify 校验明文与散列是否匹配；不匹配返回 (false, nil)，格式非法返回错误。
func Verify(plain, stored string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(stored), []byte(plain))
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
		return false, nil
	default:
		return false, fmt.Errorf("password: malformed hash: %w", err)
	}
}
