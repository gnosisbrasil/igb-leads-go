package config

import (
	"testing"
	"time"
)

func TestLoadRequiresDatabaseAndSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", "")
	if _, err := Load(); err == nil {
		t.Fatal("sem DATABASE_URL/JWT_SECRET deveria falhar")
	}
}

func TestLoadRejectsShortSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_SECRET", "curto")
	if _, err := Load(); err == nil {
		t.Fatal("JWT_SECRET curto deveria falhar")
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_SECRET", "segredo-com-mais-de-32-caracteres-ok")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != "3000" || cfg.JWTExpiresIn != time.Hour {
		t.Fatalf("defaults: %+v", cfg)
	}
	if cfg.JWTRefreshExpiresIn != 7*24*time.Hour {
		t.Fatalf("refresh expiry = %v", cfg.JWTRefreshExpiresIn)
	}
	if cfg.JWTRefreshSecret != "segredo-com-mais-de-32-caracteres-ok_refresh" {
		t.Fatalf("refresh secret = %q", cfg.JWTRefreshSecret)
	}
	if cfg.EfiSandbox != true || cfg.BcryptCost != 10 {
		t.Fatalf("flags: %+v", cfg)
	}
}

func TestEfiConfiguredMatrix(t *testing.T) {
	sandbox := &Config{EfiSandbox: true, EfiClientID: "id", EfiClientSecret: "s"}
	if !sandbox.EfiConfigured() {
		t.Fatal("sandbox com credenciais deveria configurar")
	}
	prod := &Config{EfiSandbox: false, EfiClientID: "id", EfiClientSecret: "s"}
	if prod.EfiConfigured() {
		t.Fatal("produção sem certificado/chave não deveria configurar")
	}
	prod.EfiCertBase64 = "cert"
	prod.EfiPixKey = "chave"
	if !prod.EfiConfigured() {
		t.Fatal("produção completa deveria configurar")
	}
}
