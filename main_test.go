package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cockroachdb/cockroachdb-mcp-server/config"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/require"
)

func TestNewHTTPMux(t *testing.T) {
	const token = "s3cret-test-token"
	upstreamHit := false
	upstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamHit = true
		w.WriteHeader(http.StatusOK)
	})

	type muxCase struct {
		name            string
		method, path    string
		authHeader      string
		wantStatus      int
		wantBody        string
		wantUpstreamHit bool
	}
	run := func(t *testing.T, mux http.Handler, cases []muxCase) {
		t.Helper()
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				upstreamHit = false
				req := httptest.NewRequest(tc.method, tc.path, nil)
				if tc.authHeader != "" {
					req.Header.Set("Authorization", tc.authHeader)
				}
				rec := httptest.NewRecorder()
				mux.ServeHTTP(rec, req)
				require.Equal(t, tc.wantStatus, rec.Code)
				if tc.wantBody != "" {
					require.Equal(t, tc.wantBody, rec.Body.String())
				}
				require.Equal(t, tc.wantUpstreamHit, upstreamHit)
			})
		}
	}

	t.Run("with bearer token", func(t *testing.T) {
		mux := newHTTPMux(token, upstream)
		run(t, mux, []muxCase{
			{"healthz bypasses auth", http.MethodGet, "/healthz", "", http.StatusOK, "ok", false},
			{"ready bypasses auth", http.MethodGet, "/ready", "", http.StatusOK, "ok", false},
			{"health is reachable even with a token (no-op)", http.MethodGet, "/healthz", "Bearer " + token, http.StatusOK, "ok", false},
			{"healthz HEAD is allowed", http.MethodHead, "/healthz", "", http.StatusOK, "", false},
			{"healthz POST is rejected", http.MethodPost, "/healthz", "", http.StatusMethodNotAllowed, "method not allowed\n", false},
			{"ready DELETE is rejected", http.MethodDelete, "/ready", "", http.StatusMethodNotAllowed, "method not allowed\n", false},
			{"root path without token is 401", http.MethodPost, "/", "", http.StatusUnauthorized, "unauthorized\n", false},
			{"root path with wrong token is 401", http.MethodPost, "/", "Bearer nope", http.StatusUnauthorized, "unauthorized\n", false},
			{"root path with valid token reaches upstream", http.MethodPost, "/", "Bearer " + token, http.StatusOK, "", true},
		})
	})

	t.Run("without bearer token (AllowNoBearer)", func(t *testing.T) {
		mux := newHTTPMux("", upstream)
		run(t, mux, []muxCase{
			{"healthz still works", http.MethodGet, "/healthz", "", http.StatusOK, "ok", false},
			{"ready still works", http.MethodGet, "/ready", "", http.StatusOK, "ok", false},
			{"healthz POST is still rejected", http.MethodPost, "/healthz", "", http.StatusMethodNotAllowed, "method not allowed\n", false},
			{"root path without auth header reaches upstream", http.MethodPost, "/", "", http.StatusOK, "", true},
			{"root path with stray auth header is ignored and reaches upstream", http.MethodPost, "/", "Bearer whatever", http.StatusOK, "", true},
		})
	})
}

// TestRunHTTPShutdown verifies runHTTP returns cleanly when its context is
// canceled, and that the listener is released so the port can be rebound.
func TestRunHTTPShutdown(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())

	cfg := &config.Config{
		Transport:         config.TransportHTTP,
		HTTPListenAddr:    addr,
		BearerToken:       "s3cret",
		AllowInsecureHTTP: true,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runHTTP(ctx, nil, cfg)
	}()

	require.Eventually(t, func() bool {
		c, derr := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if derr != nil {
			return false
		}
		_ = c.Close()
		return true
	}, 2*time.Second, 25*time.Millisecond, "server never started listening")

	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(shutdownPeriod + time.Second):
		t.Fatal("runHTTP did not return after context cancel")
	}
}

func TestConfigErrorMessage(t *testing.T) {
	err := errors.Newf("no database connection configured: either set %s to a connection string", "CRDB_DATABASE_URL")
	msg := configErrorMessage(err)

	t.Run("mentions server name and cause", func(t *testing.T) {
		require.Contains(t, msg, serverName)
		require.Contains(t, msg, "invalid configuration")
		require.Contains(t, msg, err.Error())
	})

	t.Run("points at the README configuration docs", func(t *testing.T) {
		require.Contains(t, msg, "https://github.com/cockroachdb/cockroachdb-mcp-server#configuration")
	})

	t.Run("carries no stack trace or structured noise", func(t *testing.T) {
		require.NotContains(t, msg, "stack trace")
		require.NotContains(t, msg, "errorVerbose")
		require.NotContains(t, msg, `"level"`)
	})
}
