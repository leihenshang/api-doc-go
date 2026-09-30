package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"apidoc/internal/model"
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
	Token string     `json:"token"`
	Info  TokenResp  `json:"info"`
}

func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// CreateToken 生成 PAT（明文只返回一次，落库只存 hash）。
func (s *Service) CreateToken(ctx context.Context, userID uint64, req *CreateTokenReq) (*CreateTokenResp, error) {
	if req == nil || req.Name == "" {
		return nil, errors.New("名称必填")
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	raw := "pat_" + hex.EncodeToString(buf)
	e := &model.UserToken{
		UserID:    userID,
		Name:      req.Name,
		TokenHash: hashToken(raw),
		Scopes:    req.Scopes,
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

// RevokeToken 吊销。
func (s *Service) RevokeToken(ctx context.Context, userID, id uint64) error {
	return s.repo.RevokeUserToken(ctx, userID, id)
}

// VerifyToken 校验 PAT 明文，返回 userID。
func (s *Service) VerifyToken(ctx context.Context, raw string) (uint64, error) {
	e, err := s.repo.GetUserTokenByHash(ctx, hashToken(raw))
	if err != nil || e == nil {
		return 0, errors.New("令牌无效")
	}
	if e.Revoked == 1 {
		return 0, errors.New("令牌已吊销")
	}
	if !e.ExpireAt.IsZero() && e.ExpireAt.Before(time.Now()) {
		return 0, errors.New("令牌已过期")
	}
	_ = s.repo.TouchUserToken(ctx, e.ID)
	return e.UserID, nil
}

func tokenInfo(e *model.UserToken) TokenResp {
	return TokenResp{
		ID: e.ID, Name: e.Name, Scopes: e.Scopes,
		ExpireAt: e.ExpireAt, LastUsed: e.LastUsed, Revoked: e.Revoked,
		CreatedAt: e.CreateTime,
	}
}
