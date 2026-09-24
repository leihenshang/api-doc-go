package jwt

import (
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
