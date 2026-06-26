package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

const testDSN = "postgresql://root@localhost:26257/defaultdb?sslmode=require"

func TestNewAdapterPasswordAuth(t *testing.T) {
	const baseDSN = "postgres://root@127.0.0.1:1/defaultdb?sslmode=disable&connect_timeout=1"
	const inlinePasswordDSN = "postgres://root:s3cret@127.0.0.1:1/defaultdb?sslmode=disable&connect_timeout=1"
	const rejectMsg = "password-based auth is disabled"

	t.Run("rejects inline password without opt-in", func(t *testing.T) {
		clearPGEnv(t)
		_, err := NewAdapter(context.Background(), Config{DSN: inlinePasswordDSN})
		require.Error(t, err, "expected error for inline password without opt-in")
		require.Containsf(t, err.Error(), rejectMsg, "error should flag password-auth rejection: %v", err)
	})

	t.Run("rejects PGPASSWORD env without opt-in", func(t *testing.T) {
		clearPGEnv(t)
		t.Setenv("PGPASSWORD", "s3cret")
		_, err := NewAdapter(context.Background(), Config{DSN: baseDSN})
		require.Error(t, err, "expected error for PGPASSWORD without opt-in")
		require.Containsf(t, err.Error(), rejectMsg, "error should flag password-auth rejection: %v", err)
	})

	t.Run("inline password with opt-in proceeds past the auth gate", func(t *testing.T) {
		clearPGEnv(t)
		_, err := NewAdapter(context.Background(), Config{
			DSN:               inlinePasswordDSN,
			AllowPasswordAuth: true,
		})
		require.Error(t, err, "connect to port 1 should fail")
		require.NotContains(t, err.Error(), rejectMsg, "opt-in should bypass the password gate")
	})

	t.Run("invalid DSN surfaces parse error before password check", func(t *testing.T) {
		clearPGEnv(t)
		_, err := NewAdapter(context.Background(), Config{DSN: "::not-a-dsn::"})
		require.Error(t, err, "expected parse error")
		require.NotContains(t, err.Error(), rejectMsg, "parse error should not mention password opt-in")
	})
}

func TestBuildPoolConfig(t *testing.T) {
	t.Run("empty DSN is rejected", func(t *testing.T) {
		_, err := buildPoolConfig(Config{})
		require.Error(t, err)
	})

	t.Run("invalid DSN is rejected", func(t *testing.T) {
		_, err := buildPoolConfig(Config{DSN: "::not a url"})
		require.Error(t, err)
	})

	t.Run("application_name from DSN is preserved", func(t *testing.T) {
		pc, err := buildPoolConfig(Config{DSN: testDSN + "&application_name=custom"})
		require.NoError(t, err)
		require.Equal(t, "custom", pc.ConnConfig.RuntimeParams["application_name"])
	})

	t.Run("qos falls back to background when DSN and cfg both empty", func(t *testing.T) {
		pc, err := buildPoolConfig(Config{DSN: testDSN})
		require.NoError(t, err)
		require.Equal(t, defaultTxnQoS,
			pc.ConnConfig.RuntimeParams["default_transaction_quality_of_service"])
	})

	t.Run("cfg.TxnQoS wins over background default", func(t *testing.T) {
		pc, err := buildPoolConfig(Config{DSN: testDSN, TxnQoS: "regular"})
		require.NoError(t, err)
		require.Equal(t, "regular",
			pc.ConnConfig.RuntimeParams["default_transaction_quality_of_service"])
	})

	t.Run("DSN-supplied qos is honored when cfg.TxnQoS is empty", func(t *testing.T) {
		dsn := testDSN + "&default_transaction_quality_of_service=critical"
		pc, err := buildPoolConfig(Config{DSN: dsn})
		require.NoError(t, err)
		require.Equal(t, "critical",
			pc.ConnConfig.RuntimeParams["default_transaction_quality_of_service"])
	})

	t.Run("cfg.TxnQoS overrides a DSN-supplied value", func(t *testing.T) {
		dsn := testDSN + "&default_transaction_quality_of_service=critical"
		pc, err := buildPoolConfig(Config{DSN: dsn, TxnQoS: "regular"})
		require.NoError(t, err)
		require.Equal(t, "regular",
			pc.ConnConfig.RuntimeParams["default_transaction_quality_of_service"],
			"explicit env var wins over DSN")
	})

	t.Run("MaxConns applied when positive", func(t *testing.T) {
		pc, err := buildPoolConfig(Config{DSN: testDSN, MaxConns: 7})
		require.NoError(t, err)
		require.Equal(t, int32(7), pc.MaxConns)
	})

	t.Run("MaxConns zero leaves pgxpool default", func(t *testing.T) {
		pc, err := buildPoolConfig(Config{DSN: testDSN})
		require.NoError(t, err)
		require.Greater(t, pc.MaxConns, int32(0), "pgxpool should fill a non-zero default")
	})

	t.Run("ReadOnly installs AfterConnect hook", func(t *testing.T) {
		pc, err := buildPoolConfig(Config{DSN: testDSN, ReadOnly: true})
		require.NoError(t, err)
		require.NotNil(t, pc.AfterConnect, "ReadOnly should install AfterConnect")
	})

	t.Run("ReadOnly false leaves AfterConnect nil", func(t *testing.T) {
		pc, err := buildPoolConfig(Config{DSN: testDSN})
		require.NoError(t, err)
		require.Nil(t, pc.AfterConnect, "non-ReadOnly should not install AfterConnect")
	})
}

// clearPGEnv unsets libpq fallback env vars so a developer's shell
// (e.g. an exported PGPASSWORD) cannot leak into the test connection
// and mask or flip the password-auth assertions.
func clearPGEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"PGPASSWORD", "PGPASSFILE", "PGSERVICEFILE", "PGSERVICE",
		"PGHOST", "PGPORT", "PGUSER", "PGDATABASE",
	} {
		t.Setenv(k, "")
	}
}
