package auth

import (
	"testing"
	"time"
)

func TestPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("password123")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(hash, "password123") {
		t.Fatal("expected password to match")
	}
	if CheckPassword(hash, "wrong") {
		t.Fatal("expected wrong password to fail")
	}
}

func TestTokenRoundTrip(t *testing.T) {
	secret := "test-secret"
	tok, err := IssueToken(secret, 42, "patron", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseToken(secret, tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != 42 || claims.Role != "patron" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}
