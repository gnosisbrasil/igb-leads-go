// Package handler holds the HTTP handlers, one file per domain.
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var startedAt = time.Now()

// WriteJSON encodes v with the UTF-8 JSON content type.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// now returns the current UTC time, matching the Zulu timestamps
// the Node API emits.
func now() time.Time {
	return time.Now().UTC()
}

// WriteError encodes a {"error": msg} payload.
func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]string{"error": msg})
}

// Health mirrors GET /health from the Node API.
func Health(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			WriteJSON(w, http.StatusServiceUnavailable, map[string]any{
				"status":    "error",
				"timestamp": time.Now(),
				"uptime":    time.Since(startedAt).Seconds(),
				"database":  "disconnected",
				"error":     err.Error(),
			})
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{
			"status":    "ok",
			"timestamp": time.Now(),
			"uptime":    time.Since(startedAt).Seconds(),
			"database":  "connected",
			"env":       os.Getenv("NODE_ENV"),
		})
	}
}

// APIConfig mirrors GET /api/config from the Node API.
func APIConfig(turnstileSiteKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{
			"CLOUDFLARE_TURNSTILE_SITE_KEY": turnstileSiteKey,
		})
	}
}
