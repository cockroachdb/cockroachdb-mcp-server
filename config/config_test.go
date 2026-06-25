package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	certPath, keyPath, caPath := writeTempCerts(t)

	baseEnv := map[string]string{
		envHost:     "crdb.example.com",
		envUser:     "root",
		envCertFile: certPath,
		envKeyFile:  keyPath,
		envCAPath:   caPath,
	}

	t.Run("defaults are applied", func(t *testing.T) {
		setEnv(t, baseEnv)
		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, defaultPort, cfg.Port)
		require.Equal(t, defaultSSLMode, cfg.SSLMode)
		require.Equal(t, defaultQueryTimeout, cfg.QueryTimeout)
		require.False(t, cfg.EnableWriteQueries, "enable-write-queries should default to false")
		require.False(t, cfg.AllowPasswordAuth, "allow-password-auth should default to false")
		require.Equal(t, defaultMaxRowsCount, cfg.MaxRowsCount)
	})

	t.Run("overrides are honored", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{
			envPort:               "5432",
			envEnableWriteQueries: "true",
			envQueryTimeout:       "10s",
			envMaxRowsCount:       "500",
		})
		setEnv(t, env)
		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, 5432, cfg.Port)
		require.True(t, cfg.EnableWriteQueries, "enable-write-queries should be true")
		require.Equal(t, 10*time.Second, cfg.QueryTimeout)
		require.Equal(t, int64(500), cfg.MaxRowsCount)
	})

	t.Run("non-positive max rows count is rejected", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{envMaxRowsCount: "0"})
		setEnv(t, env)
		_, err := Load()
		require.Error(t, err, "expected error for non-positive max rows count")
	})

	t.Run("disallowed ssl mode is rejected", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{envSSLMode: "disable"})
		setEnv(t, env)
		_, err := Load()
		require.Error(t, err, "expected error for sslmode=disable")
	})

	t.Run("missing required env vars produce a single error", func(t *testing.T) {
		clearEnv(t)
		_, err := Load()
		require.Error(t, err, "expected error when required env vars are missing")
		msg := err.Error()
		for _, want := range []string{envHost, envUser, envCertFile, envKeyFile, envCAPath} {
			require.Containsf(t, msg, want, "missing env list should mention %q", want)
		}
	})

	t.Run("missing cert file is reported", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{envCertFile: "/nonexistent/path"})
		setEnv(t, env)
		_, err := Load()
		require.Error(t, err, "expected error for missing cert file")
	})

	t.Run("allow-password-auth opt-in is parsed", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{envAllowPasswordAuth: "true"})
		setEnv(t, env)
		cfg, err := Load()
		require.NoError(t, err)
		require.True(t, cfg.AllowPasswordAuth, "AllowPasswordAuth should be true")
	})

	t.Run("invalid opt-in bool is rejected", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{envAllowPasswordAuth: "not-a-bool"})
		setEnv(t, env)
		_, err := Load()
		require.Error(t, err, "expected error for invalid bool")
	})
}

func TestDSN(t *testing.T) {
	cfg := &Config{
		Host:     "crdb.example.com",
		Port:     26257,
		User:     "root",
		SSLMode:  "verify-full",
		CertFile: "/tmp/client.crt",
		KeyFile:  "/tmp/client.key",
		CAPath:   "/tmp/ca.crt",
	}

	t.Run("includes cert params", func(t *testing.T) {
		dsn := cfg.DSN()
		for _, want := range []string{
			"sslmode=verify-full",
			"sslcert=%2Ftmp%2Fclient.crt",
			"sslkey=%2Ftmp%2Fclient.key",
			"sslrootcert=%2Ftmp%2Fca.crt",
			"root@crdb.example.com:26257",
			"/defaultdb",
		} {
			require.Containsf(t, dsn, want, "DSN missing %q", want)
		}
	})

	t.Run("includes password when set", func(t *testing.T) {
		c := *cfg
		c.Password = "s3cret"
		require.Contains(t, c.DSN(), "root:s3cret@", "DSN should include user:password")
	})

	t.Run("omits sslrootcert when CA path empty", func(t *testing.T) {
		c := *cfg
		c.CAPath = ""
		c.SSLMode = "require"
		require.NotContains(t, c.DSN(), "sslrootcert", "DSN should omit sslrootcert when empty")
	})
}

func setEnv(t *testing.T, env map[string]string) {
	t.Helper()
	clearEnv(t)
	for k, v := range env {
		t.Setenv(k, v)
	}
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		envDatabaseURL, envHost, envPort, envUser, envPassword, envSSLMode,
		envCAPath, envCertFile, envKeyFile, envEnableWriteQueries, envQueryTimeout,
		envMaxRowsCount, envAllowPasswordAuth,
	} {
		t.Setenv(k, "")
	}
}

func TestLoadDatabaseURL(t *testing.T) {
	t.Run("uses URL verbatim as DSN", func(t *testing.T) {
		clearEnv(t)
		const raw = "postgresql://root@host:26257/defaultdb?sslmode=require"
		t.Setenv(envDatabaseURL, raw)
		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, raw, cfg.DSN())
	})

	t.Run("rejects URL with disallowed sslmode", func(t *testing.T) {
		clearEnv(t)
		t.Setenv(envDatabaseURL, "postgresql://root@host:26257/defaultdb?sslmode=disable")
		_, err := Load()
		require.Error(t, err, "expected error for sslmode=disable")
	})

	t.Run("rejects URL missing sslmode", func(t *testing.T) {
		clearEnv(t)
		t.Setenv(envDatabaseURL, "postgresql://root@host:26257/defaultdb")
		_, err := Load()
		require.Error(t, err, "expected error when sslmode missing")
	})

	t.Run("URL takes precedence over cert env vars", func(t *testing.T) {
		clearEnv(t)
		t.Setenv(envDatabaseURL, "postgresql://root@host:26257/defaultdb?sslmode=verify-full")
		t.Setenv(envHost, "wrong")
		t.Setenv(envCertFile, "/does/not/exist")
		cfg, err := Load()
		require.NoError(t, err)
		require.Emptyf(t, cfg.Host, "cert fields should remain empty when URL is set: %+v", cfg)
		require.Emptyf(t, cfg.CertFile, "cert fields should remain empty when URL is set: %+v", cfg)
	})
}

func mergeEnv(base, extra map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func writeTempCerts(t *testing.T) (cert, key, ca string) {
	t.Helper()
	dir := t.TempDir()
	cert = dir + "/client.crt"
	key = dir + "/client.key"
	ca = dir + "/ca.crt"
	for _, p := range []string{cert, key, ca} {
		require.NoErrorf(t, writeFile(p, "test"), "write %s", p)
	}
	return
}

func writeFile(path, contents string) error {
	return os.WriteFile(path, []byte(contents), 0o600)
}
