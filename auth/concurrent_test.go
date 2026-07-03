package auth

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMaxConcurrent(t *testing.T) {
	t.Run("limit<=0 returns the next handler unchanged", func(t *testing.T) {
		h := MaxConcurrent(0, okHandler())
		for range 50 {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
			require.Equal(t, http.StatusOK, rec.Code)
		}
	})

	t.Run("rejects with 503 when in-flight exceeds the cap", func(t *testing.T) {
		hold := make(chan struct{})
		released := make(chan struct{})
		var inFlight atomic.Int32
		var peak atomic.Int32
		var mu sync.Mutex

		next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			n := inFlight.Add(1)
			mu.Lock()
			if n > peak.Load() {
				peak.Store(n)
			}
			mu.Unlock()
			<-hold
			inFlight.Add(-1)
			w.WriteHeader(http.StatusOK)
		})
		h := MaxConcurrent(2, next)

		results := make(chan int, 5)
		var wg sync.WaitGroup
		for range 5 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
				results <- rec.Code
			}()
		}

		require.Eventually(t, func() bool { return inFlight.Load() == 2 }, time.Second, 10*time.Millisecond, "should reach 2 in-flight")
		close(hold)
		go func() { wg.Wait(); close(released) }()
		<-released
		close(results)

		var ok, busy int
		for code := range results {
			switch code {
			case http.StatusOK:
				ok++
			case http.StatusServiceUnavailable:
				busy++
			}
		}
		require.Equal(t, 2, ok, "only the cap should succeed while 3 others race the semaphore")
		require.Equal(t, 3, busy, "remaining requests must 503")
		require.LessOrEqual(t, peak.Load(), int32(2), "in-flight must never exceed cap")
	})
}
