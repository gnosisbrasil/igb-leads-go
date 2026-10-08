package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func mockMeow(t *testing.T) (*httptest.Server, *map[string]any) {
	t.Helper()
	calls := &map[string]any{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/sessions/connect", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		(*calls)["connect"] = body
		w.Write([]byte(`{"success":true,"sessionId":"regiao-x","status":"scan_qr"}`))
	})
	mux.HandleFunc("/api/sessions/regiao-x/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"success":true,"sessionId":"regiao-x","status":"connected","phone":"5511999998888","pushName":"Equipe"}`))
	})
	mux.HandleFunc("/api/sessions/regiao-x/qr", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"success":true,"sessionId":"regiao-x","status":"scan_qr","qrImage":"data:image/png;base64,AAA"}`))
	})
	mux.HandleFunc("/api/sessions/regiao-x/disconnect", func(w http.ResponseWriter, r *http.Request) {
		(*calls)["disconnect"] = true
		w.Write([]byte(`{"success":true}`))
	})
	mux.HandleFunc("/api/messages/send-text", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		(*calls)["send"] = body
		w.Write([]byte(`{"success":true}`))
	})
	return httptest.NewServer(mux), calls
}

func TestSessionIDForRegion(t *testing.T) {
	cases := map[string]string{
		"SUL":          "regiao-sul",
		"São Paulo":    "regiao-so-paulo",
		"NORTE-01":     "regiao-norte-01",
		"  ":           "regiao-equipe",
		"Vale_Do_Aço!": "regiao-vale-do-ao",
	}
	for in, want := range cases {
		if got := SessionIDForRegion(in); got != want {
			t.Fatalf("SessionIDForRegion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWhatsAppSessionFlow(t *testing.T) {
	srv, calls := mockMeow(t)
	defer srv.Close()
	c := NewWhatsAppClient(srv.URL, "key", "default")

	if err := c.Connect("regiao-x"); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if (*calls)["connect"].(map[string]any)["sessionId"] != "regiao-x" {
		t.Fatal("Connect não enviou sessionId")
	}

	st, err := c.SessionStatus("regiao-x")
	if err != nil {
		t.Fatalf("SessionStatus: %v", err)
	}
	if st.Status != "connected" || st.Phone != "5511999998888" {
		t.Fatalf("status inesperado: %+v", st)
	}

	st, qr, err := c.SessionQR("regiao-x")
	if err != nil {
		t.Fatalf("SessionQR: %v", err)
	}
	if st.Status != "scan_qr" || qr == "" {
		t.Fatalf("qr inesperado: %+v %q", st, qr)
	}

	if err := c.SendTextTo("regiao-x", "5511888887777", "oi"); err != nil {
		t.Fatalf("SendTextTo: %v", err)
	}
	sent := (*calls)["send"].(map[string]any)
	if sent["sessionId"] != "regiao-x" || sent["to"] != "5511888887777" {
		t.Fatalf("send incorreto: %v", sent)
	}

	if err := c.Disconnect("regiao-x"); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
}

func TestWhatsAppUnconfigured(t *testing.T) {
	c := NewWhatsAppClient("", "", "default")
	if c.IsConfigured() {
		t.Fatal("IsConfigured deveria ser falso")
	}
	if err := c.SendTextTo("x", "1", "oi"); err == nil {
		t.Fatal("SendTextTo sem config deveria falhar")
	}
	if _, err := c.SessionStatus("x"); err == nil {
		t.Fatal("SessionStatus sem config deveria falhar")
	}
	if _, _, err := c.SessionQR("x"); err == nil {
		t.Fatal("SessionQR sem config deveria falhar")
	}
}
