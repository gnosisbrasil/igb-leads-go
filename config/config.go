// Package config loads the service configuration from the environment.
// Fail-fast: missing required variables abort startup instead of
// silently running in a degraded or simulated mode.
//
// Variable names intentionally match the Node backend so the same
// Coolify / .env configuration keeps working.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	CORSOrigin  string
	FrontendURL string
	APIURL      string
	NodeEnv     string

	JWTExpiresIn        time.Duration
	JWTRefreshSecret    string
	JWTRefreshExpiresIn time.Duration
	BcryptCost          int

	TurnstileSiteKey string
	TurnstileSecret  string

	GoogleClientID     string
	GoogleClientSecret string
	GoogleCallbackURL  string

	EfiClientID     string
	EfiClientSecret string
	EfiCertBase64   string
	EfiPixKey       string
	EfiSandbox      bool
	EfiBaseURL      string

	MeowURL     string
	MeowAPIKey  string
	MeowSession string

	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPass     string
	SMTPSecure   bool
	SMTPFrom     string
	SMTPFromName string

	UploadDir string
}

func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// parseDuration accepts Go durations plus a "d" (days) suffix,
// mirroring the jsonwebtoken strings used by the Node backend.
func parseDuration(s string) (time.Duration, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	if n, err := strconv.Atoi(strings.TrimSuffix(s, "d")); err == nil && strings.HasSuffix(s, "d") {
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return 0, fmt.Errorf("duração inválida: %q", s)
}

// Load reads the environment. DATABASE_URL and JWT_SECRET are required;
// every other integration is optional at boot and fails loudly when used
// without its credentials.
func Load() (*Config, error) {
	var missing []string
	required := func(key string) string {
		v := strings.TrimSpace(os.Getenv(key))
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}

	cfg := &Config{
		Port:        getenv("PORT", "3000"),
		DatabaseURL: required("DATABASE_URL"),
		JWTSecret:   required("JWT_SECRET"),
		CORSOrigin:  getenv("CORS_ORIGIN", getenv("FRONTEND_URL", "http://localhost:5173")),
		FrontendURL: getenv("FRONTEND_URL", "http://localhost:5173"),
		APIURL:      getenv("API_URL", "http://localhost:3000"),
		NodeEnv:     getenv("NODE_ENV", "development"),
		BcryptCost:  10,

		TurnstileSiteKey: os.Getenv("CLOUDFLARE_TURNSTILE_SITE_KEY"),
		TurnstileSecret:  os.Getenv("CLOUDFLARE_TURNSTILE_SECRET"),

		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleCallbackURL:  os.Getenv("GOOGLE_CALLBACK_URL"),

		EfiClientID:     os.Getenv("EFI_CLIENT_ID"),
		EfiClientSecret: os.Getenv("EFI_CLIENT_SECRET"),
		EfiCertBase64:   os.Getenv("EFI_CERTIFICATE_BASE64"),
		EfiPixKey:       os.Getenv("EFI_PIX_KEY"),
		EfiSandbox:      getenv("EFI_SANDBOX", "true") != "false",
		EfiBaseURL:      os.Getenv("EFI_BASE_URL"),

		MeowURL:     strings.TrimSuffix(getenv("MEOW_URL", ""), "/"),
		MeowAPIKey:  os.Getenv("MEOW_API_KEY"),
		MeowSession: getenv("MEOW_SESSION", "default"),

		SMTPHost:     getenv("SMTP_HOST", "smtp.gmail.com"),
		SMTPUser:     os.Getenv("SMTP_USER"),
		SMTPPass:     os.Getenv("SMTP_PASSWORD"),
		SMTPSecure:   os.Getenv("SMTP_SECURE") == "true",
		SMTPFrom:     getenv("SMTP_FROM", "noreply@gnosisbrasil.com"),
		SMTPFromName: getenv("SMTP_FROM_NAME", "Sistema de Leads - Gnosis Brasil"),

		UploadDir: getenv("UPLOAD_DIR", "./uploads"),
	}
	if p := strings.TrimSpace(os.Getenv("SMTP_PORT")); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("SMTP_PORT inválido: %q", p)
		}
		cfg.SMTPPort = n
	} else {
		cfg.SMTPPort = 587
	}
	var err error
	if cfg.JWTExpiresIn, err = parseDuration(getenv("JWT_EXPIRES_IN", "1h")); err != nil {
		return nil, err
	}
	cfg.JWTRefreshSecret = getenv("JWT_REFRESH_SECRET", cfg.JWTSecret+"_refresh")
	if cfg.JWTRefreshExpiresIn, err = parseDuration(getenv("JWT_REFRESH_EXPIRES_IN", "7d")); err != nil {
		return nil, err
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("variáveis obrigatórias ausentes: %s", strings.Join(missing, ", "))
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET deve ter ao menos 32 caracteres")
	}
	return cfg, nil
}

// GoogleConfigured reports whether Google OAuth is available.
func (c *Config) GoogleConfigured() bool {
	return c.GoogleClientID != "" && c.GoogleClientSecret != ""
}

// GoogleRedirectURL is the login callback; an explicit env wins,
// otherwise the API convention path is used.
func (c *Config) GoogleRedirectURL() string {
	if c.GoogleCallbackURL != "" {
		return c.GoogleCallbackURL
	}
	return strings.TrimSuffix(c.APIURL, "/") + "/api/auth/google/callback"
}

// GoogleConnectRedirectURL is the account-linking callback.
func (c *Config) GoogleConnectRedirectURL() string {
	return strings.TrimSuffix(c.APIURL, "/") + "/api/auth/connect/google/callback"
}

// EfiConfigured reports whether Pix charges can be issued. Sandbox needs
// only client credentials; production also needs the mTLS certificate.
func (c *Config) EfiConfigured() bool {
	if c.EfiSandbox {
		return c.EfiClientID != "" && c.EfiClientSecret != ""
	}
	return c.EfiClientID != "" && c.EfiClientSecret != "" && c.EfiCertBase64 != "" && c.EfiPixKey != ""
}

// MeowConfigured reports whether WhatsApp sending is available.
func (c *Config) MeowConfigured() bool {
	return c.MeowURL != "" && c.MeowAPIKey != ""
}

// SMTPConfigured reports whether transactional email can be sent.
func (c *Config) SMTPConfigured() bool {
	return c.SMTPHost != "" && c.SMTPUser != "" && c.SMTPPass != ""
}
