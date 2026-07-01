package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestBearer(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	cases := []struct {
		name       string
		header     string
		wantStatus int
		wantBody   string
	}{
		{"valid bearer token", "Bearer secret", http.StatusOK, "ok"},
		{"missing header", "", http.StatusUnauthorized, "unauthorized\n"},
		{"wrong scheme", "Basic secret", http.StatusUnauthorized, "unauthorized\n"},
		{"wrong token", "Bearer nope", http.StatusUnauthorized, "unauthorized\n"},
		{"empty token after prefix", "Bearer ", http.StatusUnauthorized, "unauthorized\n"},
		{"case-sensitive scheme", "bearer secret", http.StatusUnauthorized, "unauthorized\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			Bearer("secret", ok).ServeHTTP(rec, req)
			require.Equal(t, tc.wantStatus, rec.Code)
			require.Equal(t, tc.wantBody, rec.Body.String())
		})
	}
}

func TestBearerLogsRejection(t *testing.T) {
	core, recorded := observer.New(zapcore.WarnLevel)
	defer zap.ReplaceGlobals(zap.New(core))()

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rec := httptest.NewRecorder()
	Bearer("secret", nil).ServeHTTP(rec, req)

	entries := recorded.All()
	require.Len(t, entries, 1)
	require.Equal(t, "auth failed", entries[0].Message)
	fields := entries[0].ContextMap()
	require.Equal(t, http.MethodGet, fields["method"])
	require.Equal(t, "/mcp", fields["path"])
}
