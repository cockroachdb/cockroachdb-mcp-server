package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestRateLimit(t *testing.T) {
	t.Run("rps<=0 lets every request through", func(t *testing.T) {
		h := RateLimit(0, 0, false, okHandler())
		for range 20 {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
			require.Equal(t, http.StatusOK, rec.Code)
		}
	})

	t.Run("burst then 429 for the same client", func(t *testing.T) {
		h := RateLimit(1, 3, false, okHandler()) // rps=1, burst=3 -> first 3 OK, 4th 429
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		for i := 1; i <= 3; i++ {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			require.Equalf(t, http.StatusOK, rec.Code, "request %d should be allowed", i)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, http.StatusTooManyRequests, rec.Code)
		require.Equal(t, "1", rec.Header().Get("Retry-After"))
	})

	t.Run("separate clients get independent buckets", func(t *testing.T) {
		h := RateLimit(1, 1, false, okHandler())
		req1 := httptest.NewRequest(http.MethodPost, "/", nil)
		req1.RemoteAddr = "192.0.2.1:1234"
		req2 := httptest.NewRequest(http.MethodPost, "/", nil)
		req2.RemoteAddr = "192.0.2.2:1234"
		rec1 := httptest.NewRecorder()
		h.ServeHTTP(rec1, req1)
		require.Equal(t, http.StatusOK, rec1.Code)
		rec2 := httptest.NewRecorder()
		h.ServeHTTP(rec2, req2)
		require.Equal(t, http.StatusOK, rec2.Code, "different client should not share rec1's bucket")
	})
}

func TestClientKey(t *testing.T) {
	t.Run("bearer token hashes to a stable key", func(t *testing.T) {
		r1 := httptest.NewRequest(http.MethodGet, "/", nil)
		r1.Header.Set("Authorization", "Bearer abc123")
		r2 := httptest.NewRequest(http.MethodGet, "/", nil)
		r2.Header.Set("Authorization", "Bearer abc123")
		r2.RemoteAddr = "different:1234"
		require.Equal(t, clientKey(r1, false), clientKey(r2, false), "same token must map to same key regardless of addr")
		require.NotContains(t, clientKey(r1, false), "abc123", "key must not leak the raw token")
	})

	t.Run("XFF is ignored unless trusted", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.1")
		r.RemoteAddr = "192.0.2.9:1234"
		require.Equal(t, "ip:192.0.2.9", clientKey(r, false),
			"forgeable XFF must not influence the key by default")
	})

	t.Run("trusted XFF uses the rightmost entry, not the client-supplied first", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("X-Forwarded-For", "6.6.6.6, 203.0.113.5")
		r.RemoteAddr = "10.0.0.1:1234"
		require.Equal(t, "xff:203.0.113.5", clientKey(r, true))
	})

	t.Run("falls back to RemoteAddr without port", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "198.51.100.7:54321"
		require.Equal(t, "ip:198.51.100.7", clientKey(r, false))
	})

	t.Run("IPv6 RemoteAddr keeps the full host", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "[2001:db8::1]:54321"
		require.Equal(t, "ip:2001:db8::1", clientKey(r, false))
	})
}
