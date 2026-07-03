package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// maxLimiterKeys caps the per-key limiter map so a misbehaving or attacking
// client cannot grow it without bound. Oldest entry is evicted on overflow.
const maxLimiterKeys = 10000

// RateLimit returns middleware that enforces a per-client token-bucket rate.
// Clients are keyed by bearer-token hash when present, otherwise by client
// address (see clientKey). Returns next unchanged if rps <= 0. A non-positive
// burst is normalized to max(1, 2*rps).
func RateLimit(rps float64, burst int, trustXFF bool, next http.Handler) http.Handler {
	if rps <= 0 {
		return next
	}
	if burst <= 0 {
		burst = max(1, int(2*rps))
	}
	set := newLimiterSet(rate.Limit(rps), burst)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !set.allow(clientKey(r, trustXFF)) {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type limiterSet struct {
	mu    sync.Mutex
	keys  map[string]*limiterEntry
	limit rate.Limit
	burst int
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newLimiterSet(limit rate.Limit, burst int) *limiterSet {
	return &limiterSet{
		keys:  make(map[string]*limiterEntry),
		limit: limit,
		burst: burst,
	}
}

func (s *limiterSet) allow(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.keys[key]
	if !ok {
		if len(s.keys) >= maxLimiterKeys {
			s.evictOldestLocked()
		}
		e = &limiterEntry{limiter: rate.NewLimiter(s.limit, s.burst)}
		s.keys[key] = e
	}
	e.lastSeen = time.Now()
	return e.limiter.Allow()
}

func (s *limiterSet) evictOldestLocked() {
	var oldKey string
	var oldTime time.Time
	for k, v := range s.keys {
		if oldKey == "" || v.lastSeen.Before(oldTime) {
			oldKey, oldTime = k, v.lastSeen
		}
	}
	delete(s.keys, oldKey)
}

// clientKey returns a stable per-client identifier, preferring an
// authenticated identity. Bearer tokens are hashed so the key cannot be
// reversed from a memory dump. X-Forwarded-For is used only when trustXFF is
// set (the header is client-forgeable), and only its rightmost entry, which
// is the one appended by the nearest trusted proxy.
func clientKey(r *http.Request, trustXFF bool) string {
	if a := r.Header.Get("Authorization"); strings.HasPrefix(a, "Bearer ") {
		h := sha256.Sum256([]byte(a[len("Bearer "):]))
		return "tok:" + hex.EncodeToString(h[:8])
	}
	if xff := r.Header.Get("X-Forwarded-For"); trustXFF && xff != "" {
		parts := strings.Split(xff, ",")
		return "xff:" + strings.TrimSpace(parts[len(parts)-1])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return "ip:" + host
}
