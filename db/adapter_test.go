package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewAdapterPasswordAuth(t *testing.T) {
	const baseDSN = "postgres://root@127.0.0.1:1/defaultdb?sslmode=disable&connect_timeout=1"
	const inlinePasswordDSN = "postgres://root:s3cret@127.0.0.1:1/defaultdb?sslmode=disable&connect_timeout=1"
	const rejectMsg = "password-based auth is disabled"

	t.Run("rejects inline password without opt-in", func(t *testing.T) {
		clearPasswordEnv(t)
		_, err := NewAdapter(context.Background(), Config{DSN: inlinePasswordDSN})
		require.Error(t, err, "expected error for inline password without opt-in")
		require.Containsf(t, err.Error(), rejectMsg, "error should flag password-auth rejection: %v", err)
	})

	t.Run("rejects PGPASSWORD env without opt-in", func(t *testing.T) {
		clearPasswordEnv(t)
		t.Setenv("PGPASSWORD", "s3cret")
		_, err := NewAdapter(context.Background(), Config{DSN: baseDSN})
		require.Error(t, err, "expected error for PGPASSWORD without opt-in")
		require.Containsf(t, err.Error(), rejectMsg, "error should flag password-auth rejection: %v", err)
	})

	t.Run("inline password with opt-in proceeds past the auth gate", func(t *testing.T) {
		clearPasswordEnv(t)
		_, err := NewAdapter(context.Background(), Config{
			DSN:               inlinePasswordDSN,
			AllowPasswordAuth: true,
		})
		require.Error(t, err, "connect to port 1 should fail")
		require.NotContains(t, err.Error(), rejectMsg, "opt-in should bypass the password gate")
	})

	t.Run("invalid DSN surfaces parse error before password check", func(t *testing.T) {
		clearPasswordEnv(t)
		_, err := NewAdapter(context.Background(), Config{DSN: "::not-a-dsn::"})
		require.Error(t, err, "expected parse error")
		require.NotContains(t, err.Error(), rejectMsg, "parse error should not mention password opt-in")
	})

	t.Run("empty DSN is rejected", func(t *testing.T) {
		clearPasswordEnv(t)
		_, err := NewAdapter(context.Background(), Config{})
		require.Error(t, err, "expected error for empty DSN")
	})
}

func clearPasswordEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"PGPASSWORD", "PGPASSFILE", "PGSERVICEFILE", "PGSERVICE",
		"PGHOST", "PGPORT", "PGUSER", "PGDATABASE",
	} {
		t.Setenv(k, "")
	}
}
