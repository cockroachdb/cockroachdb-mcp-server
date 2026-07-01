// Package auth provides HTTP authentication middleware for the MCP server.
package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/cockroachdb/cockroachdb-mcp-server/logging"
	"go.uber.org/zap"
)

const bearerPrefix = "Bearer "

// Bearer wraps next, accepting only requests whose Authorization header
// carries the configured bearer token. Both sides are SHA-256 hashed before
// the constant-time compare so the comparator always sees fixed-length input,
// regardless of attacker-controlled token length.
func Bearer(token string, next http.Handler) http.Handler {
	if token == "" {
		panic("auth.Bearer: token must not be empty")
	}
	expected := sha256.Sum256([]byte(token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		// Strict case-sensitive scheme match; RFC 7235 allows insensitive.
		if !strings.HasPrefix(auth, bearerPrefix) {
			reject(w, r)
			return
		}
		got := sha256.Sum256([]byte(auth[len(bearerPrefix):]))
		if subtle.ConstantTimeCompare(got[:], expected[:]) != 1 {
			reject(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func reject(w http.ResponseWriter, r *http.Request) {
	logging.L(r.Context()).Warn("auth failed",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("remote", r.RemoteAddr),
	)
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}
