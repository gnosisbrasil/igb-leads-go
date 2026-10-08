package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

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

// SendText delivers a text message to the digits-only number.
func (c *WhatsAppClient) SendText(to, text string) error {
	if !c.IsConfigured() {
		return fmt.Errorf("WhatsApp API não configurada")
	}
	payload, _ := json.Marshal(map[string]string{
		"sessionId": c.session, "to": to, "text": text,
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
