package jwt

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestGenerateParseRoundTrip(t *testing.T) {
	tok, err := Generate(42, 2, "apidoc", "secret", time.Hour)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	claims, err := Parse(tok, "secret")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if claims.UserID != 42 || claims.Type != 2 {
		t.Fatalf("claims mismatch: %+v", claims)
	}
}

func TestParseRejectsWrongSecret(t *testing.T) {
	tok, _ := Generate(1, 1, "apidoc", "secret", time.Hour)
	if _, err := Parse(tok, "other"); err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestParseRejectsTamperedPayload(t *testing.T) {
	tok, _ := Generate(1, 1, "apidoc", "secret", time.Hour)
	parts := strings.Split(tok, ".")
	tampered := parts[0] + "." + parts[1] + "x." + parts[2]
	if _, err := Parse(tampered, "secret"); err == nil {
		t.Fatal("expected error for tampered token")
	}
}

func TestParseRejectsExpired(t *testing.T) {
	tok, _ := Generate(1, 1, "apidoc", "secret", -time.Minute)
	if _, err := Parse(tok, "secret"); err == nil {
		t.Fatal("expected error for expired token")
	}
}

// TestParseRejectsWrongAlg P2-6：header 里的 alg 必须是 HS256。
// 不做这一步时，只要将来有人按 header 选算法（很自然的「支持 RS256」演进），
// `{"alg":"none"}` 就会立刻变成签名绕过。
func TestParseRejectsWrongAlg(t *testing.T) {
	tok, err := Generate(1, 1, Issuer, "secret", time.Hour)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	parts := strings.Split(tok, ".")
	for _, alg := range []string{"none", "RS256", "hs256", ""} {
		head := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"` + alg + `","typ":"JWT"}`))
		if _, err := Parse(head+"."+parts[1]+"."+parts[2], "secret"); err == nil {
			t.Fatalf("alg=%q 应被拒绝", alg)
		}
	}
}

// TestParseRejectsForeignIssuer iss 非空时必须等于本系统的 Issuer。
func TestParseRejectsForeignIssuer(t *testing.T) {
	tok, err := Generate(1, 1, "someone-else", "secret", time.Hour)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if _, err := Parse(tok, "secret"); err == nil {
		t.Fatal("外部 iss 应被拒绝")
	}
	// 空 iss 视为旧令牌，仍需可用（历史兼容）
	legacy, err := Generate(1, 1, "", "secret", time.Hour)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if _, err := Parse(legacy, "secret"); err != nil {
		t.Fatalf("空 iss 的旧令牌应继续可用: %v", err)
	}
}

// TestParseRejectsMissingExp 没有 exp 的令牌一律拒绝（否则等于永不过期）。
func TestParseRejectsMissingExp(t *testing.T) {
	head := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	body := base64.RawURLEncoding.EncodeToString([]byte(`{"uid":1,"typ":1,"iss":"apidoc"}`))
	signing := head + "." + body
	tok := signing + "." + sign(signing, "secret")
	if _, err := Parse(tok, "secret"); err == nil {
		t.Fatal("缺少 exp 的令牌应被拒绝")
	}
}
