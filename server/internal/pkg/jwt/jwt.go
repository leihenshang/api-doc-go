package jwt

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// ErrInvalid 表示令牌格式错误、签名不匹配或已过期。
var ErrInvalid = errors.New("jwt: invalid token")

// Claims 最小化 JWT 载荷。
type Claims struct {
	UserID uint64 `json:"uid"`
	Type   int8   `json:"typ"`
	Issuer string `json:"iss"`
	Iat    int64  `json:"iat"`
	Exp    int64  `json:"exp"`
}

const headerJSON = `{"alg":"HS256","typ":"JWT"}`

// Generate 生成 HS256 令牌，有效期 ttl。secret 为空时返回错误。
func Generate(userID uint64, typ int8, issuer, secret string, ttl time.Duration) (string, error) {
	if secret == "" {
		return "", errors.New("jwt: empty secret")
	}
	now := time.Now()
	claims := Claims{
		UserID: userID,
		Type:   typ,
		Issuer: issuer,
		Iat:    now.Unix(),
		Exp:    now.Add(ttl).Unix(),
	}
	head := base64.RawURLEncoding.EncodeToString([]byte(headerJSON))
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	signing := head + "." + body
	return signing + "." + sign(signing, secret), nil
}

// Parse 校验签名与过期时间，成功返回 Claims。
func Parse(token, secret string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalid
	}
	signing := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(sign(signing, secret)), []byte(parts[2])) {
		return nil, ErrInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalid
	}
	var claims Claims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return nil, ErrInvalid
	}
	if time.Now().Unix() > claims.Exp {
		return nil, ErrInvalid
	}
	return &claims, nil
}

// sign 计算 HMAC-SHA256 并 base64url 编码。
func sign(data, secret string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
