package config

import (
	"os"
	"strings"
	"testing"
	"time"
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
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Port != defaultPort {
			t.Fatalf("port: got %d, want %d", cfg.Port, defaultPort)
		}
		if cfg.SSLMode != defaultSSLMode {
			t.Fatalf("ssl mode: got %q, want %q", cfg.SSLMode, defaultSSLMode)
		}
		if cfg.QueryTimeout != defaultQueryTimeout {
			t.Fatalf("query timeout: got %v, want %v", cfg.QueryTimeout, defaultQueryTimeout)
		}
		if cfg.EnableWriteQueries {
			t.Fatal("enable-write-queries should default to false")
		}
		if cfg.MaxRowsCount != defaultMaxRowsCount {
			t.Fatalf("max rows count: got %d, want %d", cfg.MaxRowsCount, defaultMaxRowsCount)
		}
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
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Port != 5432 {
			t.Fatalf("port: got %d, want 5432", cfg.Port)
		}
		if !cfg.EnableWriteQueries {
			t.Fatal("enable-write-queries should be true")
		}
		if cfg.QueryTimeout != 10*time.Second {
			t.Fatalf("query timeout: got %v", cfg.QueryTimeout)
		}
		if cfg.MaxRowsCount != 500 {
			t.Fatalf("max rows count: got %d, want 500", cfg.MaxRowsCount)
		}
	})

	t.Run("non-positive max rows count is rejected", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{envMaxRowsCount: "0"})
		setEnv(t, env)
		if _, err := Load(); err == nil {
			t.Fatal("expected error for non-positive max rows count")
		}
	})

	t.Run("disallowed ssl mode is rejected", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{envSSLMode: "disable"})
		setEnv(t, env)
		if _, err := Load(); err == nil {
			t.Fatal("expected error for sslmode=disable")
		}
	})

	t.Run("missing required env vars produce a single error", func(t *testing.T) {
		clearEnv(t)
		_, err := Load()
		if err == nil {
			t.Fatal("expected error when required env vars are missing")
		}
		msg := err.Error()
		for _, want := range []string{envHost, envUser, envCertFile, envKeyFile, envCAPath} {
			if !strings.Contains(msg, want) {
				t.Fatalf("missing env list should mention %q: %v", want, err)
			}
		}
	})

	t.Run("missing cert file is reported", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{envCertFile: "/nonexistent/path"})
		setEnv(t, env)
		if _, err := Load(); err == nil {
			t.Fatal("expected error for missing cert file")
		}
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
			if !strings.Contains(dsn, want) {
				t.Fatalf("DSN missing %q: %s", want, dsn)
			}
		}
	})

	t.Run("includes password when set", func(t *testing.T) {
		c := *cfg
		c.Password = "s3cret"
		if !strings.Contains(c.DSN(), "root:s3cret@") {
			t.Fatalf("DSN should include user:password: %s", c.DSN())
		}
	})

	t.Run("omits sslrootcert when CA path empty", func(t *testing.T) {
		c := *cfg
		c.CAPath = ""
		c.SSLMode = "require"
		if strings.Contains(c.DSN(), "sslrootcert") {
			t.Fatalf("DSN should omit sslrootcert when empty: %s", c.DSN())
		}
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
		envMaxRowsCount,
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
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.DSN() != raw {
			t.Fatalf("DSN: got %q, want %q", cfg.DSN(), raw)
		}
	})

	t.Run("rejects URL with disallowed sslmode", func(t *testing.T) {
		clearEnv(t)
		t.Setenv(envDatabaseURL, "postgresql://root@host:26257/defaultdb?sslmode=disable")
		if _, err := Load(); err == nil {
			t.Fatal("expected error for sslmode=disable")
		}
	})

	t.Run("rejects URL missing sslmode", func(t *testing.T) {
		clearEnv(t)
		t.Setenv(envDatabaseURL, "postgresql://root@host:26257/defaultdb")
		if _, err := Load(); err == nil {
			t.Fatal("expected error when sslmode missing")
		}
	})

	t.Run("URL takes precedence over cert env vars", func(t *testing.T) {
		clearEnv(t)
		t.Setenv(envDatabaseURL, "postgresql://root@host:26257/defaultdb?sslmode=verify-full")
		t.Setenv(envHost, "wrong")
		t.Setenv(envCertFile, "/does/not/exist")
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Host != "" || cfg.CertFile != "" {
			t.Fatalf("cert fields should remain empty when URL is set: %+v", cfg)
		}
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
		if err := writeFile(p, "test"); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	return
}

func writeFile(path, contents string) error {
	return os.WriteFile(path, []byte(contents), 0o600)
}
