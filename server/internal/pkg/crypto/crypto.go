// Package crypto 提供对称加密工具，用于外部数据源口令等敏感字段的存储加密。
// 算法为 AES-256-GCM（标准库实现），密钥由环境变量 SECRET_ENCRYPT_KEY 派生（sha256）。
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// ErrKeyMissing 表示未配置加密密钥。
var ErrKeyMissing = errors.New("crypto: empty encrypt key")

// Encrypt 以 key 派生 AES-256-GCM 密钥加密明文，输出 base64(nonce|ciphertext)。
// key 为空返回 ErrKeyMissing。
func Encrypt(plain, key string) (string, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: nonce: %w", err)
	}
	sum := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(sum), nil
}

// Decrypt 解密 Encrypt 产物。密文被篡改或密钥不匹配时返回错误。
func Decrypt(encoded, key string) (string, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("crypto: decode: %w", err)
	}
	n := gcm.NonceSize()
	if len(raw) < n {
		return "", errors.New("crypto: ciphertext too short")
	}
	plain, err := gcm.Open(nil, raw[:n], raw[n:], nil)
	if err != nil {
		return "", fmt.Errorf("crypto: decrypt: %w", err)
	}
	return string(plain), nil
}

// newGCM 由任意长度密钥字符串派生 AES-256-GCM。
func newGCM(key string) (cipher.AEAD, error) {
	if key == "" {
		return nil, ErrKeyMissing
	}
	sum := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, fmt.Errorf("crypto: aes: %w", err)
	}
	return cipher.NewGCM(block)
}
