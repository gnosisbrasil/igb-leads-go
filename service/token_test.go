package service

import (
	"testing"
	"time"
)

func testTokens() *TokenService {
	return NewTokenService("segredo-de-teste-com-32-chars-ok!", time.Hour, "segredo-refresh-de-teste-ok!", 7*24*time.Hour)
}

func TestAccessRoundtrip(t *testing.T) {
	s := testTokens()
	raw, err := s.SignAccess("user-1", "a@b.c", "admin")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	claims, err := s.VerifyAccess(raw)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.ID != "user-1" || claims.Email != "a@b.c" || claims.Role != "admin" {
		t.Fatalf("claims = %+v", claims)
	}
}

func TestAccessWrongSecret(t *testing.T) {
	s := testTokens()
	raw, _ := s.SignAccess("user-1", "a@b.c", "admin")
	other := NewTokenService("outro-segredo-de-32-chars-xxxxx!", time.Hour, "x", time.Hour)
	if _, err := other.VerifyAccess(raw); err != ErrTokenInvalid {
		t.Fatalf("err = %v, want ErrTokenInvalid", err)
	}
}

func TestAccessExpired(t *testing.T) {
	s := NewTokenService("segredo-de-teste-com-32-chars-ok!", -time.Hour, "x", time.Hour)
	raw, _ := s.SignAccess("user-1", "a@b.c", "admin")
	if _, err := s.VerifyAccess(raw); err != ErrTokenExpired {
		t.Fatalf("err = %v, want ErrTokenExpired", err)
	}
}

func TestAccessGarbage(t *testing.T) {
	if _, err := testTokens().VerifyAccess("não-é-token"); err != ErrTokenInvalid {
		t.Fatalf("err = %v, want ErrTokenInvalid", err)
	}
}

func TestRefreshRoundtrip(t *testing.T) {
	s := testTokens()
	raw, err := s.SignRefresh("user-1")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	claims, err := s.VerifyRefresh(raw)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.ID != "user-1" || claims.Type != "refresh" {
		t.Fatalf("claims = %+v", claims)
	}
}

func TestRefreshRejectsAccessToken(t *testing.T) {
	s := testTokens()
	access, _ := s.SignAccess("user-1", "a@b.c", "admin")
	if _, err := s.VerifyRefresh(access); err != ErrTokenInvalid {
		t.Fatalf("err = %v, want ErrTokenInvalid", err)
	}
}

// bcryptjsVector was generated with Node bcryptjs (hashSync, cost 10)
// for "SenhaForte123" — production passwords must keep verifying.
const bcryptjsVector = "$2b$10$llUC6LXcpiNWl.gyMDfAHOHV5Fi7VlVrxJStckKN238DubmH9TFJW"

func TestCheckPasswordBcryptjsVector(t *testing.T) {
	if !CheckPassword("SenhaForte123", bcryptjsVector) {
		t.Fatal("Go deve validar hash gerado pelo bcryptjs")
	}
	if CheckPassword("SenhaErrada", bcryptjsVector) {
		t.Fatal("senha errada não deve validar")
	}
}

func TestHashPasswordRoundtrip(t *testing.T) {
	hash, err := HashPassword("OutraSenha456", 10)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !CheckPassword("OutraSenha456", hash) {
		t.Fatal("hash próprio deve validar")
	}
}
