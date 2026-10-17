package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"apidoc/internal/model"
	"apidoc/internal/pkg/crypto"
)

// TokenResp PAT 列表项（不含明文）。
type TokenResp struct {
	ID        uint64    `json:"id"`
	Name      string    `json:"name"`
	Scopes    string    `json:"scopes"`
	ExpireAt  time.Time `json:"expire_at"`
	LastUsed  time.Time `json:"last_used"`
	Revoked   int8      `json:"revoked"`
	CreatedAt time.Time `json:"create_time"`
}

// CreateTokenReq 创建 PAT。
type CreateTokenReq struct {
	Name   string `json:"name"`
	Scopes string `json:"scopes"`
	Days   int    `json:"days"` // 有效天数；0 = 不过期
}

// CreateTokenResp 创建结果（明文只此一次）。
type CreateTokenResp struct {
	Token string    `json:"token"`
	Info  TokenResp `json:"info"`
}

// tokenDigest 令牌摘要 = HMAC-SHA256(服务端密钥, 令牌明文)。
//
// 用 HMAC 而不是裸 sha256（P1-12）：PAT 本身是 256 位随机串，裸 sha256 已不可逆，
// 但加上服务端密钥后「脱库」不再等于「可离线比对」——密钥在配置文件/环境变量里，
// 不随数据库一起泄露。摘要只做等值查找，不参与任何需要可逆的场景。
func (s *Service) tokenDigest(raw string) string {
	m := hmac.New(sha256.New, []byte(s.cfg.JWT.Secret))
	m.Write([]byte(raw))
	return hex.EncodeToString(m.Sum(nil))
}

// legacyTokenDigest 旧版纯 sha256 摘要，仅用于兼容已有令牌：
// 命中后由 VerifyToken 就地升级为 tokenDigest，无需让用户重建令牌。
func legacyTokenDigest(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// CreateToken 生成 PAT：明文返回一次 + 加密存储（S15），落库另存 hash 供鉴权。
func (s *Service) CreateToken(ctx context.Context, userID uint64, req *CreateTokenReq) (*CreateTokenResp, error) {
	if req == nil || req.Name == "" {
		return nil, errors.New("名称必填")
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	raw := "pat_" + hex.EncodeToString(buf)
	cipher, err := crypto.Encrypt(s.cfg.JWT.Secret, raw)
	if err != nil {
		return nil, err
	}
	e := &model.UserToken{
		UserID:      userID,
		Name:        req.Name,
		TokenHash:   s.tokenDigest(raw),
		TokenCipher: cipher,
		Scopes:      req.Scopes,
	}
	if req.Days > 0 {
		e.ExpireAt = time.Now().AddDate(0, 0, req.Days)
	}
	if err := s.repo.CreateUserToken(ctx, e); err != nil {
		return nil, err
	}
	return &CreateTokenResp{
		Token: raw,
		Info:  tokenInfo(e),
	}, nil
}

// ListTokens 我的 PAT 列表。
func (s *Service) ListTokens(ctx context.Context, userID uint64) ([]TokenResp, error) {
	list, err := s.repo.ListUserTokens(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]TokenResp, 0, len(list))
	for i := range list {
		out = append(out, tokenInfo(&list[i]))
	}
	return out, nil
}

// DeleteToken 删除（硬删，不可恢复）：行删除后客户端同步即被拒。
func (s *Service) DeleteToken(ctx context.Context, userID, id uint64) error {
	return s.repo.DeleteUserToken(ctx, userID, id)
}

// RevealToken 查看本人 PAT 明文（服务端解密，S15）：换浏览器/清缓存仍可查看。
// 旧令牌密文为空或已删除时返回错误。
func (s *Service) RevealToken(ctx context.Context, userID, id uint64) (string, error) {
	e, err := s.repo.GetUserTokenByID(ctx, userID, id)
	if err != nil || e == nil {
		return "", errors.New("令牌不存在")
	}
	if e.Revoked == 1 {
		return "", errors.New("令牌已删除")
	}
	if e.TokenCipher == "" {
		return "", errors.New("该令牌未保存明文（旧令牌），请删除后重新创建")
	}
	raw, err := crypto.Decrypt(s.cfg.JWT.Secret, e.TokenCipher)
	if err != nil {
		return "", err
	}
	return raw, nil
}

// VerifyToken 校验 PAT 明文，返回 userID。
// VerifyToken 校验 PAT：返回归属用户 ID 与其类型。
// 除令牌本身的吊销/过期外，还校验用户存在且状态正常 —— 此前被禁用用户的 PAT 仍可用，
// 且 PAT 一律被降级为普通用户（管理员持 PAT 会丢失管理权限）。
func (s *Service) VerifyToken(ctx context.Context, raw string) (uint64, int8, error) {
	if raw == "" {
		return 0, 0, errors.New("令牌无效")
	}
	e, err := s.repo.GetUserTokenByHash(ctx, s.tokenDigest(raw))
	if err != nil || e == nil {
		// 兼容旧记录（裸 sha256）：命中即就地 upgrade，用户无感。
		legacy, lerr := s.repo.GetUserTokenByHash(ctx, legacyTokenDigest(raw))
		if lerr != nil || legacy == nil {
			return 0, 0, errors.New("令牌无效")
		}
		_ = s.repo.UpdateUserTokenHash(ctx, legacy.ID, s.tokenDigest(raw))
		e = legacy
	}
	if e.Revoked == 1 {
		return 0, 0, errors.New("令牌已吊销")
	}
	if !e.ExpireAt.IsZero() && e.ExpireAt.Before(time.Now()) {
		return 0, 0, errors.New("令牌已过期")
	}
	u, err := s.repo.UserByID(ctx, e.UserID)
	if err != nil || u == nil {
		return 0, 0, errors.New("令牌对应用户不存在")
	}
	if u.State != model.UserStateNormal {
		return 0, 0, errors.New("用户已被禁用")
	}
	_ = s.repo.TouchUserToken(ctx, e.ID)
	return e.UserID, u.Type, nil
}

func tokenInfo(e *model.UserToken) TokenResp {
	return TokenResp{
		ID: e.ID, Name: e.Name, Scopes: e.Scopes,
		ExpireAt: e.ExpireAt, LastUsed: e.LastUsed, Revoked: e.Revoked,
		CreatedAt: e.CreateTime,
	}
}
