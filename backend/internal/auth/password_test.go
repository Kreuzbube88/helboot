package auth

import (
	"strings"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("hash has unexpected format: %s", hash)
	}

	ok, err := VerifyPassword("correct horse battery staple", hash)
	if err != nil || !ok {
		t.Errorf("correct password rejected (ok=%v, err=%v)", ok, err)
	}

	ok, err = VerifyPassword("wrong password", hash)
	if err != nil {
		t.Errorf("VerifyPassword error: %v", err)
	}
	if ok {
		t.Error("wrong password accepted")
	}
}

func TestHashesAreSalted(t *testing.T) {
	h1, _ := HashPassword("same password")
	h2, _ := HashPassword("same password")
	if h1 == h2 {
		t.Error("two hashes of the same password are identical; salt missing")
	}
}

func TestVerifyPasswordRejectsGarbage(t *testing.T) {
	if _, err := VerifyPassword("x", "not-a-hash"); err == nil {
		t.Error("expected error for malformed hash")
	}
}

func TestNewToken(t *testing.T) {
	t1, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	t2, _ := NewToken()
	if t1 == t2 {
		t.Error("tokens are not unique")
	}
	if len(t1) < 40 {
		t.Errorf("token too short: %d chars", len(t1))
	}
}
