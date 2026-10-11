// Package crypto 提供对称加解密（AES-256-GCM），用于 PAT 明文的安全存储（S15）。
// 密钥由任意 secret 派生：SHA-256(secret) 取前 32 字节，避免依赖外部密钥文件。
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

// Encrypt 用 secret 加密 plaintext，返回 base64(nonce(12B) + ciphertext)。
// 每次加密生成随机 nonce；同一明文两次加密结果不同。
func Encrypt(secret, plaintext string) (string, error) {
	block, err := aes.NewCipher(keyOf(secret))
	if err != nil {
		return "", fmt.Errorf("crypto: 初始化 AES: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: 初始化 GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: 生成 nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt 解密 Encrypt 的输出；密文非法（篡改/密钥不符）时返回错误。
func Decrypt(secret, ciphertext string) (string, error) {
	block, err := aes.NewCipher(keyOf(secret))
	if err != nil {
		return "", fmt.Errorf("crypto: 初始化 AES: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: 初始化 GCM: %w", err)
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("crypto: 解码密文: %w", err)
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("crypto: 密文长度非法")
	}
	nonce, body := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return "", fmt.Errorf("crypto: 解密失败（密钥不符或密文被篡改）: %w", err)
	}
	return string(plain), nil
}

// keyOf 从任意 secret 派生 32 字节 AES 密钥。
func keyOf(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}
