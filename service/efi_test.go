package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func mockEfi(t *testing.T) (*httptest.Server, *int64, *map[string]any) {
	t.Helper()
	var oauthCalls int64
	var lastPut map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("POST /oauth/token", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&oauthCalls, 1)
		u, p, ok := r.BasicAuth()
		if !ok || u != "id" || p != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "expires_in": 3600, "token_type": "Bearer"})
	})
	mux.HandleFunc("PUT /v2/cob/{txid}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&lastPut)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"txid": r.PathValue("txid"), "pixCopiaECola": "000201payload",
		})
	})
	mux.HandleFunc("GET /v2/cob/{txid}", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"txid": r.PathValue("txid"), "status": "CONCLUIDA"})
	})
	return httptest.NewServer(mux), &oauthCalls, &lastPut
}

func TestEfiCreatePixCharge(t *testing.T) {
	srv, oauthCalls, lastPut := mockEfi(t)
	defer srv.Close()
	c := NewEfiClient(EfiConfig{ClientID: "id", ClientSecret: "secret", PixKey: "chave", Sandbox: true, BaseURL: srv.URL})
	charge, err := c.CreatePixCharge(context.Background(), 1500.5, "Campanha: X", "TXID123", Payer{Name: "Maria"})
	if err != nil {
		t.Fatalf("charge: %v", err)
	}
	if charge.Txid != "TXID123" || charge.QRCode != "000201payload" {
		t.Fatalf("charge = %+v", charge)
	}
	put := *lastPut
	if put["chave"] != "chave" {
		t.Fatalf("chave = %v", put["chave"])
	}
	valor := put["valor"].(map[string]any)
	if valor["original"] != "1500.50" {
		t.Fatalf("valor = %v", valor)
	}
	devedor := put["devedor"].(map[string]any)
	if devedor["nome"] != "Maria" {
		t.Fatalf("devedor = %v", devedor)
	}
	if atomic.LoadInt64(oauthCalls) != 1 {
		t.Fatalf("oauth calls = %d, want 1", *oauthCalls)
	}
	// Second charge reuses the cached token.
	if _, err := c.CreatePixCharge(context.Background(), 10, "y", "TX2", Payer{}); err != nil {
		t.Fatalf("charge2: %v", err)
	}
	if atomic.LoadInt64(oauthCalls) != 1 {
		t.Fatalf("oauth calls = %d, want cached 1", *oauthCalls)
	}
	if (*lastPut)["devedor"].(map[string]any)["nome"] != "Pagador" {
		t.Fatalf("default devedor = %v", (*lastPut)["devedor"])
	}
	// Description truncates at 140 chars.
	long := strings.Repeat("x", 200)
	if _, err := c.CreatePixCharge(context.Background(), 10, long, "TX3", Payer{}); err != nil {
		t.Fatalf("charge3: %v", err)
	}
	if got := (*lastPut)["solicitacaoPagador"].(string); len(got) != 140 {
		t.Fatalf("solicitacao len = %d", len(got))
	}
}

func TestEfiVerifyPayment(t *testing.T) {
	srv, _, _ := mockEfi(t)
	defer srv.Close()
	c := NewEfiClient(EfiConfig{ClientID: "id", ClientSecret: "secret", PixKey: "chave", Sandbox: true, BaseURL: srv.URL})
	st, err := c.VerifyPayment(context.Background(), "TXID123")
	if err != nil || st == nil || st.Status != "CONCLUIDA" {
		t.Fatalf("status = %+v, err = %v", st, err)
	}
}

func TestEfiChargeFailsUpstream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "t", "expires_in": 60})
			return
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"nome":"cob_invalida"}`))
	}))
	defer srv.Close()
	c := NewEfiClient(EfiConfig{ClientID: "id", ClientSecret: "secret", PixKey: "chave", Sandbox: true, BaseURL: srv.URL})
	if _, err := c.CreatePixCharge(context.Background(), 10, "x", "T", Payer{}); err == nil {
		t.Fatal("upstream 422 deveria falhar alto")
	}
	st, err := c.VerifyPayment(context.Background(), "T")
	if err != nil || st != nil {
		t.Fatalf("verify em erro = %+v, %v; want nil,nil (mantém pendente)", st, err)
	}
}

func TestGetCobStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "t", "expires_in": 3600})
			return
		}
		switch r.URL.Path {
		case "/v2/cob/PAGA":
			_ = json.NewEncoder(w).Encode(map[string]any{"txid": "PAGA", "status": "CONCLUIDA"})
		case "/v2/cob/QUEBRADA":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := NewEfiClient(EfiConfig{ClientID: "id", ClientSecret: "secret", PixKey: "chave", Sandbox: true, BaseURL: srv.URL})

	st, err := c.GetCobStatus(context.Background(), "PAGA")
	if err != nil || st != "CONCLUIDA" {
		t.Fatalf("paga = %q, %v", st, err)
	}
	if _, err := c.GetCobStatus(context.Background(), "FORJADA"); !errors.Is(err, ErrCobNotFound) {
		t.Fatalf("forjada: err = %v, want ErrCobNotFound", err)
	}
	if _, err := c.GetCobStatus(context.Background(), "QUEBRADA"); err == nil || errors.Is(err, ErrCobNotFound) {
		t.Fatalf("quebrada: err = %v, want erro genérico (retry)", err)
	}
	// O poll continua engolindo tudo (mantém pendente).
	if st, err := c.VerifyPayment(context.Background(), "FORJADA"); err != nil || st != nil {
		t.Fatalf("verify 404 = %+v, %v; want nil,nil", st, err)
	}
	if st, err := c.VerifyPayment(context.Background(), "QUEBRADA"); err != nil || st != nil {
		t.Fatalf("verify 500 = %+v, %v; want nil,nil", st, err)
	}
}
