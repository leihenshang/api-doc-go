package crypto

import "testing"

func TestRoundTrip(t *testing.T) {
	t.Parallel()
	const secret = "test-jwt-secret-change-me"
	const plain = "pat_0123456789abcdef0123456789abcdef"
	enc, err := Encrypt(secret, plain)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if enc == plain {
		t.Fatal("密文不应等于明文")
	}
	got, err := Decrypt(secret, enc)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if got != plain {
		t.Fatalf("往返不一致: got %q want %q", got, plain)
	}
}

func TestNonceRandom(t *testing.T) {
	t.Parallel()
	const secret = "s"
	a, _ := Encrypt(secret, "pat_x")
	b, _ := Encrypt(secret, "pat_x")
	if a == b {
		t.Fatal("同一明文两次加密结果应不同（随机 nonce）")
	}
}

func TestWrongSecretFails(t *testing.T) {
	t.Parallel()
	enc, err := Encrypt("key-a", "pat_x")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if _, err := Decrypt("key-b", enc); err == nil {
		t.Fatal("密钥不符应解密失败")
	}
}

func TestTamperedFails(t *testing.T) {
	t.Parallel()
	enc, err := Encrypt("key", "pat_x")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if _, err := Decrypt("key", enc+"AA"); err == nil {
		t.Fatal("篡改密文应解密失败")
	}
}
