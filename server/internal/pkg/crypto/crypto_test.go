package crypto

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	enc, err := Encrypt("s3cret", "key-a")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if enc == "s3cret" {
		t.Fatal("ciphertext equals plaintext")
	}
	got, err := Decrypt(enc, "key-a")
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if got != "s3cret" {
		t.Fatalf("got %q", got)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	enc, _ := Encrypt("x", "key-a")
	if _, err := Decrypt(enc, "key-b"); err == nil {
		t.Fatal("expected error for wrong key")
	}
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	enc, _ := Encrypt("x", "key-a")
	tampered := enc[:len(enc)-2] + "AA"
	if _, err := Decrypt(tampered, "key-a"); err == nil {
		t.Fatal("expected error for tampered ciphertext")
	}
}

func TestEmptyKeyRejected(t *testing.T) {
	if _, err := Encrypt("x", ""); err == nil {
		t.Fatal("expected error for empty key")
	}
}
