// Package fileutil 上传文件校验与命名：
// 扩展名白名单、文件头嗅探（防改名绕过）、随机文件名、响应 Content-Type 映射与访问签名。
package fileutil

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"path/filepath"
	"strings"
)

// DefaultAllowExt 默认允许的上传扩展名（图片与 PDF）。
// **不含 svg**：SVG 可内嵌脚本，而 /uploads 允许公开读取，存在 XSS 风险。
var DefaultAllowExt = []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".ico", ".pdf"}

// ParseAllowExt 解析逗号分隔的扩展名配置（补点、转小写、去重）；raw 为空时返回默认值。
func ParseAllowExt(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultAllowExt
	}
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, part := range strings.Split(raw, ",") {
		ext := strings.ToLower(strings.TrimSpace(part))
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		if _, dup := seen[ext]; dup {
			continue
		}
		seen[ext] = struct{}{}
		out = append(out, ext)
	}
	if len(out) == 0 {
		return DefaultAllowExt
	}
	return out
}

// NormalizeExt 取小写扩展名（含点）；无扩展名或为点文件时返回空串。
func NormalizeExt(name string) string {
	ext := strings.ToLower(filepath.Ext(filepath.Base(name)))
	if ext == "." {
		return ""
	}
	return ext
}

// Allowed 判断扩展名是否在白名单内（ext 需为小写带点形式）。
func Allowed(ext string, allow []string) bool {
	if ext == "" {
		return false
	}
	for _, a := range allow {
		if a == ext {
			return true
		}
	}
	return false
}

// SniffAllowed 校验文件头真实类型与扩展名同族，防止把 .html/.js 改名为 .png 上传。
// head 为文件前 512 字节（不足亦可）。
func SniffAllowed(head []byte, ext string) bool {
	kind := http.DetectContentType(head)
	switch ext {
	case ".pdf":
		return strings.HasPrefix(kind, "application/pdf")
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".ico":
		// 具体子类型在不同 Go 版本可能不同（如 ico 为 image/x-icon / image/vnd.microsoft.icon），
		// 这里只要求归入 image/* 家族。
		return strings.HasPrefix(kind, "image/")
	default:
		// 白名单外的扩展名一律拒绝（调用方已先做扩展名校验）
		return false
	}
}

// RandomName 生成随机文件名（32 位十六进制 + 扩展名），避免文件名可枚举。
func RandomName(ext string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf) + ext, nil
}

// ContentTypeOf 按扩展名返回响应用的 Content-Type。
// **不使用** http.DetectContentType：避免把内容误判成 text/html 而在浏览器里执行。
func ContentTypeOf(ext string) string {
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".ico":
		return "image/x-icon"
	case ".pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}

// Sign 计算文件访问签名（HMAC-SHA256，取前 16 字节 hex）；secret 为空时返回空串（表示不启用签名）。
func Sign(name, secret string) string {
	if secret == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(name))
	return hex.EncodeToString(mac.Sum(nil)[:16])
}
