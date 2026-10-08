package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAllowUpToMax(t *testing.T) {
	l := NewRateLimiter(time.Minute, 3)
	for i := range 3 {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		r.RemoteAddr = "10.0.0.1:1234"
		if !l.Allow(w, r) {
			t.Fatalf("hit %d bloqueado antes do limite", i+1)
		}
		if got := w.Header().Get("RateLimit-Limit"); got != "3" {
			t.Fatalf("RateLimit-Limit = %q", got)
		}
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	r.RemoteAddr = "10.0.0.1:1234"
	if l.Allow(w, r) {
		t.Fatal("hit além do limite deveria bloquear")
	}
	if got := w.Header().Get("RateLimit-Remaining"); got != "0" {
		t.Fatalf("RateLimit-Remaining = %q", got)
	}
	if got := w.Header().Get("RateLimit-Reset"); got == "" {
		t.Fatal("RateLimit-Reset ausente")
	}
}

func TestWindowReset(t *testing.T) {
	l := NewRateLimiter(30*time.Millisecond, 1)
	mkreq := func() (*httptest.ResponseRecorder, *http.Request) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/x", nil)
		r.RemoteAddr = "10.0.0.2:1234"
		return w, r
	}
	w, r := mkreq()
	if !l.Allow(w, r) {
		t.Fatal("primeiro hit deveria passar")
	}
	w, r = mkreq()
	if l.Allow(w, r) {
		t.Fatal("segundo hit deveria bloquear")
	}
	time.Sleep(50 * time.Millisecond)
	w, r = mkreq()
	if !l.Allow(w, r) {
		t.Fatal("após a janela deveria passar de novo")
	}
}

func TestMiddleware429Shape(t *testing.T) {
	l := NewRateLimiter(time.Minute, 1)
	h := l.Middleware("Muitas tentativas.", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.RemoteAddr = "10.0.0.3:1234"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("code = %d, want 429", w.Code)
	}
	want := `{"error":"Muitas tentativas."}`
	if w.Body.String() != want {
		t.Fatalf("body = %q, want %q", w.Body.String(), want)
	}
}

func TestClientIPFirstForwarded(t *testing.T) {
	mk := func(xff, real string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/x", nil)
		r.RemoteAddr = "10.9.9.9:1234"
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		if real != "" {
			r.Header.Set("X-Real-Ip", real)
		}
		return r
	}
	if got := clientIP(mk("1.2.3.4, 5.6.7.8", "")); got != "1.2.3.4" {
		t.Fatalf("XFF multiplo = %q, want primeiro", got)
	}
	if got := clientIP(mk("", "9.9.9.9")); got != "9.9.9.9" {
		t.Fatalf("X-Real-Ip = %q", got)
	}
	if got := clientIP(mk("", "")); got != "10.9.9.9" {
		t.Fatalf("RemoteAddr = %q", got)
	}
}

func TestBucketsSeparadosPorIP(t *testing.T) {
	l := NewRateLimiter(time.Minute, 1)
	hit := func(ip string) bool {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/x", nil)
		r.RemoteAddr = "10.9.9.9:1234"
		r.Header.Set("X-Forwarded-For", ip)
		return l.Allow(w, r)
	}
	if !hit("1.1.1.1") {
		t.Fatal("primeiro IP deveria passar")
	}
	if !hit("2.2.2.2") {
		t.Fatal("IP diferente não deveria consumir o mesmo balde")
	}
	if hit("1.1.1.1") {
		t.Fatal("mesmo IP deveria bloquear no segundo hit")
	}
}
