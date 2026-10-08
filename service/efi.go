package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/pkcs12"
)

// EfiClient talks to the Efí Pix API: OAuth client_credentials,
// PUT /v2/cob/:txid to charge, GET /v2/cob/:txid to poll.
// Production requires the mTLS .p12 certificate; sandbox does not.
type EfiClient struct {
	clientID     string
	clientSecret string
	pixKey       string
	sandbox      bool
	baseURL      string
	http         *http.Client

	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

// EfiConfig carries the Efí settings (plus a test override for the base URL).
type EfiConfig struct {
	ClientID     string
	ClientSecret string
	CertBase64   string
	PixKey       string
	Sandbox      bool
	BaseURL      string // empty = default host; tests point it at a mock
}

func NewEfiClient(cfg EfiConfig) *EfiClient {
	base := cfg.BaseURL
	if base == "" {
		if cfg.Sandbox {
			base = "https://pix-h.api.efipay.com.br"
		} else {
			base = "https://pix.api.efipay.com.br"
		}
	}
	c := &EfiClient{
		clientID: cfg.ClientID, clientSecret: cfg.ClientSecret,
		pixKey: cfg.PixKey, sandbox: cfg.Sandbox, baseURL: base,
		http: &http.Client{Timeout: 30 * time.Second},
	}
	if cfg.CertBase64 != "" {
		cert, err := decodeP12(cfg.CertBase64)
		if err != nil {
			log.Printf("EFI: erro ao decodificar certificado: %v", err)
		} else {
			c.http.Transport = &http.Transport{
				TLSClientConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
			}
			log.Print("EFI: cliente HTTPS com certificado mTLS configurado")
		}
	} else {
		log.Print("EFI: certificado não configurado. Chamadas PIX podem falhar.")
	}
	return c
}

func decodeP12(b64 string) (tls.Certificate, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return tls.Certificate{}, err
	}
	key, cert, err := pkcs12.Decode(raw, "")
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{cert.Raw}, PrivateKey: key, Leaf: cert}, nil
}

// IsConfigured mirrors the Node rule: sandbox needs client credentials,
// production also needs the certificate and the Pix key.
func (c *EfiClient) IsConfigured() bool {
	if c.sandbox {
		return c.clientID != "" && c.clientSecret != ""
	}
	return c.clientID != "" && c.clientSecret != "" && c.pixKey != ""
}

func (c *EfiClient) IsSandbox() bool { return c.sandbox }

// PixCharge is the created immediate charge.
type PixCharge struct {
	Txid      string
	QRCode    string // pixCopiaECola payload
	ExpiresAt time.Time
}

// authToken fetches and caches the OAuth token (60s early refresh).
func (c *EfiClient) authToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	if c.token != "" && time.Now().Before(c.tokenExp) {
		defer c.mu.Unlock()
		return c.token, nil
	}
	c.mu.Unlock()

	body, _ := json.Marshal(map[string]string{"grant_type": "client_credentials"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/oauth/token", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.clientID, c.clientSecret)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("efi oauth: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("EFI: erro ao obter token: %s", raw)
		return "", fmt.Errorf("falha na autenticação com EFI")
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("efi oauth: %w", err)
	}
	c.mu.Lock()
	c.token = out.AccessToken
	c.tokenExp = time.Now().Add(time.Duration(out.ExpiresIn-60) * time.Second)
	c.mu.Unlock()
	return out.AccessToken, nil
}

func (c *EfiClient) authed(ctx context.Context, method, path string, payload any) ([]byte, int, error) {
	token, err := c.authToken(ctx)
	if err != nil {
		return nil, 0, err
	}
	var rdr *bytes.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return raw, resp.StatusCode, nil
}

// Payer carries optional devedor data.
type Payer struct {
	Name string
	CPF  string
	CNPJ string
}

// CreatePixCharge issues an immediate charge (24h expiry).
func (c *EfiClient) CreatePixCharge(ctx context.Context, amount float64, description, txid string, payer Payer) (*PixCharge, error) {
	if c.pixKey == "" {
		return nil, fmt.Errorf("EFI_PIX_KEY não configurada")
	}
	name := payer.Name
	if name == "" {
		name = "Pagador"
	}
	devedor := map[string]string{"nome": name}
	if payer.CPF != "" {
		devedor["cpf"] = payer.CPF
	}
	if payer.CNPJ != "" {
		devedor["cnpj"] = payer.CNPJ
	}
	if len(description) > 140 {
		description = description[:140]
	}
	payload := map[string]any{
		"calendario":         map[string]int{"expiracao": 86400},
		"devedor":            devedor,
		"valor":              map[string]string{"original": formatMoney(amount)},
		"chave":              c.pixKey,
		"solicitacaoPagador": description,
	}
	raw, status, err := c.authed(ctx, http.MethodPut, "/v2/cob/"+txid, payload)
	if err != nil {
		log.Printf("EFI: erro ao criar cobrança Pix: %v", err)
		return nil, fmt.Errorf("falha ao gerar pagamento Pix")
	}
	if status < 200 || status >= 300 {
		log.Printf("EFI: erro ao criar cobrança Pix: %s", raw)
		return nil, fmt.Errorf("falha ao gerar pagamento Pix")
	}
	var out struct {
		Txid          string `json:"txid"`
		PixCopiaECola string `json:"pixCopiaECola"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.PixCopiaECola == "" {
		log.Printf("EFI: resposta inesperada: %s", raw)
		return nil, fmt.Errorf("falha ao gerar pagamento Pix")
	}
	return &PixCharge{Txid: out.Txid, QRCode: out.PixCopiaECola, ExpiresAt: time.Now().Add(24 * time.Hour)}, nil
}

// CobStatus is the polled charge status.
type CobStatus struct {
	Status string
	Raw    []byte
}

// ErrCobNotFound reports an Efí 404: no charge exists for the txid.
var ErrCobNotFound = errors.New("cobrança não encontrada na Efí")

// GetCobStatus fetches GET /v2/cob/:txid, distinguishing "no such charge"
// (ErrCobNotFound) from transport/upstream failures.
func (c *EfiClient) GetCobStatus(ctx context.Context, txid string) (string, error) {
	raw, status, err := c.authed(ctx, http.MethodGet, "/v2/cob/"+txid, nil)
	if err != nil {
		return "", err
	}
	if status == http.StatusNotFound {
		return "", ErrCobNotFound
	}
	if status < 200 || status >= 300 {
		log.Printf("EFI: erro ao consultar cobrança: %s", raw)
		return "", fmt.Errorf("efi cob: status %d", status)
	}
	var out struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.Status == "" {
		return "", fmt.Errorf("efi cob: resposta inesperada")
	}
	return out.Status, nil
}

// VerifyPayment polls GET /v2/cob/:txid; nil means "unknown, keep pending".
func (c *EfiClient) VerifyPayment(ctx context.Context, txid string) (*CobStatus, error) {
	st, err := c.GetCobStatus(ctx, txid)
	if err != nil {
		if !errors.Is(err, ErrCobNotFound) {
			log.Printf("EFI: erro ao verificar pagamento: %v", err)
		}
		return nil, nil
	}
	return &CobStatus{Status: st}, nil
}

// RegisterWebhook binds the notification URL to a Pix key (one-time setup).
func (c *EfiClient) RegisterWebhook(ctx context.Context, pixKey, webhookURL string) error {
	raw, status, err := c.authed(ctx, http.MethodPut, "/v2/webhook/"+pixKey, map[string]string{"webhookUrl": webhookURL})
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("efi webhook: %s", raw)
	}
	return nil
}

func formatMoney(amount float64) string {
	return fmt.Sprintf("%.2f", amount)
}
