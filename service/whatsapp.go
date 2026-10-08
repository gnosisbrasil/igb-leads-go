package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SessionStatus mirrors the meow session payload.
type SessionStatus struct {
	SessionID string `json:"sessionId"`
	Status    string `json:"status"`
	Phone     string `json:"phone"`
	PushName  string `json:"pushName"`
}

// SessionIDForRegion derives a stable meow session id from the region code.
func SessionIDForRegion(code string) string {
	s := strings.ToLower(strings.TrimSpace(code))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == ' ':
			b.WriteByte('-')
		}
	}
	id := strings.Trim(b.String(), "-")
	if id == "" {
		id = "equipe"
	}
	return "regiao-" + id
}

// WhatsAppClient sends through the unofficial meow API
// (POST /api/messages/send-text), replacing the Meta Cloud API.
type WhatsAppClient struct {
	baseURL string
	apiKey  string
	session string
	client  *http.Client
}

func NewWhatsAppClient(baseURL, apiKey, session string) *WhatsAppClient {
	return &WhatsAppClient{
		baseURL: baseURL, apiKey: apiKey, session: session,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// IsConfigured reports whether sending is available.
func (c *WhatsAppClient) IsConfigured() bool {
	return c.baseURL != "" && c.apiKey != ""
}

// DefaultSession returns the fallback session from configuration.
func (c *WhatsAppClient) DefaultSession() string { return c.session }

// SendText delivers a text message through the default session.
func (c *WhatsAppClient) SendText(to, text string) error {
	return c.SendTextTo(c.session, to, text)
}

// SendTextTo delivers a text message through an explicit session.
func (c *WhatsAppClient) SendTextTo(session, to, text string) error {
	if !c.IsConfigured() {
		return fmt.Errorf("WhatsApp API não configurada")
	}
	payload, _ := json.Marshal(map[string]string{
		"sessionId": session, "to": to, "text": text,
	})
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/messages/send-text", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("meow: status %d", resp.StatusCode)
	}
	return nil
}

func (c *WhatsAppClient) getJSON(path string, out any) error {
	if !c.IsConfigured() {
		return fmt.Errorf("WhatsApp API não configurada")
	}
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("meow: status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// SessionStatus queries the meow status of a session.
func (c *WhatsAppClient) SessionStatus(session string) (SessionStatus, error) {
	var st SessionStatus
	if err := c.getJSON("/api/sessions/"+session+"/status", &st); err != nil {
		return SessionStatus{}, err
	}
	return st, nil
}

// SessionQR returns the pairing status plus the QR image data URL.
func (c *WhatsAppClient) SessionQR(session string) (SessionStatus, string, error) {
	var body struct {
		SessionStatus
		QRImage string `json:"qrImage"`
	}
	if err := c.getJSON("/api/sessions/"+session+"/qr", &body); err != nil {
		return SessionStatus{}, "", err
	}
	return body.SessionStatus, body.QRImage, nil
}

// Connect creates the session on meow (starts QR generation).
func (c *WhatsAppClient) Connect(session string) error {
	if !c.IsConfigured() {
		return fmt.Errorf("WhatsApp API não configurada")
	}
	payload, _ := json.Marshal(map[string]string{"sessionId": session})
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/sessions/connect", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("meow: status %d", resp.StatusCode)
	}
	return nil
}

// Disconnect drops the meow session connection.
func (c *WhatsAppClient) Disconnect(session string) error {
	if !c.IsConfigured() {
		return fmt.Errorf("WhatsApp API não configurada")
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/sessions/"+session+"/disconnect", nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("meow: status %d", resp.StatusCode)
	}
	return nil
}
