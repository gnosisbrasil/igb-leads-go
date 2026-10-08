package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// CaptchaVerifier validates Cloudflare Turnstile tokens.
// Verification is skipped in development or without a secret,
// mirroring the Node middleware.
type CaptchaVerifier struct {
	secret    string
	skip      bool
	client    *http.Client
	verifyURL string
}

func NewCaptchaVerifier(secret, nodeEnv string) *CaptchaVerifier {
	return &CaptchaVerifier{
		secret:    secret,
		skip:      nodeEnv == "development" || secret == "",
		client:    &http.Client{Timeout: 10 * time.Second},
		verifyURL: "https://challenges.cloudflare.com/turnstile/v0/siteverify",
	}
}

// Verify checks the token from the cfTurnstileToken body field.
func (c *CaptchaVerifier) Verify(token string) error {
	if c.skip {
		return nil
	}
	if token == "" {
		return errCaptchaRequired
	}
	form := url.Values{"secret": {c.secret}, "response": {token}}
	resp, err := c.client.PostForm(c.verifyURL, form)
	if err != nil {
		return fmt.Errorf("turnstile: %w", err)
	}
	defer resp.Body.Close()
	var out struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("turnstile: resposta inválida")
	}
	if !out.Success {
		return errCaptchaInvalid
	}
	return nil
}

var (
	errCaptchaRequired = captchaError("CAPTCHA obrigatório")
	errCaptchaInvalid  = captchaError("CAPTCHA inválido")
)

type captchaError string

func (e captchaError) Error() string { return string(e) }

// IsCaptchaError reports whether err is a client captcha failure
// (400) as opposed to a verification outage (500).
func IsCaptchaError(err error) bool {
	_, ok := err.(captchaError)
	return ok || strings.HasPrefix(err.Error(), "CAPTCHA")
}
