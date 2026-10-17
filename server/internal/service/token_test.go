package service

// P1-12：PAT 摘要用 HMAC（带服务端密钥）而不是裸 sha256，并兼容旧记录。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"apidoc/internal/model"
)

func TestTokenDigestIsPeppered(t *testing.T) {
	f := newVarFixture(t)
	ctx := context.Background()
	res, err := f.s.CreateToken(ctx, f.admin.ID, &CreateTokenReq{Name: "ci"})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	// 落库的摘要必须与「裸 sha256」不同：说明带上了服务端密钥（pepper）。
	raw := sha256.Sum256([]byte(res.Token))
	legacy := hex.EncodeToString(raw[:])
	list, err := f.s.repo.ListUserTokens(ctx, f.admin.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListUserTokens: %v (%d)", err, len(list))
	}
	if list[0].TokenHash == legacy {
		t.Fatal("摘要不应等于裸 sha256（应带服务端密钥）")
	}
	if list[0].TokenHash != f.s.tokenDigest(res.Token) {
		t.Fatal("落库摘要应与 tokenDigest 一致")
	}

	// 正常校验路径
	uid, typ, err := f.s.VerifyToken(ctx, res.Token)
	if err != nil || uid != f.admin.ID || typ != f.admin.Type {
		t.Fatalf("VerifyToken: uid=%d typ=%d err=%v", uid, typ, err)
	}
	// 错令牌 / 空令牌必须被拒
	if _, _, err := f.s.VerifyToken(ctx, "pat_deadbeef"); err == nil {
		t.Fatal("错误令牌应被拒")
	}
	if _, _, err := f.s.VerifyToken(ctx, ""); err == nil {
		t.Fatal("空令牌应被拒")
	}
}

func TestVerifyTokenUpgradesLegacyHash(t *testing.T) {
	f := newVarFixture(t)
	ctx := context.Background()
	// 手工造一条「旧版记录」：摘要为裸 sha256，密文留空（旧数据形态）
	const rawToken = "pat_legacy_0123456789"
	sum := sha256.Sum256([]byte(rawToken))
	legacyHash := hex.EncodeToString(sum[:])
	e := &model.UserToken{UserID: f.admin.ID, Name: "legacy", TokenHash: legacyHash}
	if err := f.s.repo.CreateUserToken(ctx, e); err != nil {
		t.Fatalf("CreateUserToken: %v", err)
	}

	// 旧令牌必须仍可用
	uid, _, err := f.s.VerifyToken(ctx, rawToken)
	if err != nil || uid != f.admin.ID {
		t.Fatalf("旧令牌应仍可校验: uid=%d err=%v", uid, err)
	}
	// 且校验后摘要被就地升级为 HMAC
	var got model.UserToken
	if err := f.s.repo.DB().Where("id = ?", e.ID).First(&got).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.TokenHash == legacyHash {
		t.Fatal("校验旧令牌后应把摘要升级为 HMAC 形态")
	}
	if got.TokenHash != f.s.tokenDigest(rawToken) {
		t.Fatal("升级后的摘要应等于 tokenDigest")
	}
	// 升级后仍能正常校验
	if _, _, err := f.s.VerifyToken(ctx, rawToken); err != nil {
		t.Fatalf("升级后应仍可校验: %v", err)
	}
}
