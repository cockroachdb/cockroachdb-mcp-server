package auth

import "net/http"

// MaxConcurrent returns middleware that caps in-flight requests via a
// semaphore. Excess requests are rejected with 503 (not blocked) so a
// burst of slow handlers cannot exhaust the http.Server worker pool.
// Returns next unchanged if limit <= 0.
func MaxConcurrent(limit int, next http.Handler) http.Handler {
	if limit <= 0 {
		return next
	}
	sem := make(chan struct{}, limit)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
			next.ServeHTTP(w, r)
		default:
			w.Header().Set("Retry-After", "1")
			http.Error(w, "server busy", http.StatusServiceUnavailable)
		}
	})
}
