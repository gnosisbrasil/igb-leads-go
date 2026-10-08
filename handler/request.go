package handler

import "net/http"

// requestIP prefers the leftmost forwarded address (Traefik sets
// X-Forwarded-For), falling back to the peer address.
func requestIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return fwd
	}
	return r.RemoteAddr
}
