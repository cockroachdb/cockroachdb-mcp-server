package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
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
