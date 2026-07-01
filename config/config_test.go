package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

const testBearerToken = "test-bearer-token"

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
		require.Equal(t, defaultMaxConns, cfg.MaxConns)
		require.Empty(t, cfg.TxnQoS, "txn qos is empty unless env var is explicit; adapter applies the fallback")
		require.Equal(t, defaultLogLevel, cfg.LogLevel)
		require.Empty(t, cfg.LogPath, "log path defaults to empty (stderr)")
	})

	t.Run("log path is captured and validated as writable", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "mcp.log")
		env := mergeEnv(baseEnv, map[string]string{envLogPath: path})
		setEnv(t, env)
		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, path, cfg.LogPath)
	})

	t.Run("log path '-' is accepted (stderr alias)", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{envLogPath: "-"})
		setEnv(t, env)
		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, "-", cfg.LogPath)
	})

	t.Run("unwritable log path is rejected at load time", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{envLogPath: "/nonexistent/dir/mcp.log"})
		setEnv(t, env)
		_, err := Load()
		require.Error(t, err)
		require.Contains(t, err.Error(), envLogPath)
	})

	t.Run("log level override is honored", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{envLogLevel: "warn"})
		setEnv(t, env)
		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, zapcore.WarnLevel, cfg.LogLevel)
	})

	t.Run("invalid log level is rejected", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{envLogLevel: "trace"})
		setEnv(t, env)
		_, err := Load()
		require.Error(t, err, "expected error for invalid log level")
		require.Contains(t, err.Error(), "debug, info, warn, or error")
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

	t.Run("http transport requires bearer token", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{envTransport: TransportHTTP})
		setEnv(t, env)
		_, err := Load()
		require.Error(t, err, "expected error when http transport is set without bearer token")
		require.Containsf(t, err.Error(), envBearerToken, "error should mention bearer token env: %v", err)
	})

	t.Run("http transport with short bearer token is rejected", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{
			envTransport:         TransportHTTP,
			envBearerToken:       "too-short",
			envAllowInsecureHTTP: "true",
		})
		setEnv(t, env)
		_, err := Load()
		require.Error(t, err)
		require.Contains(t, err.Error(), "at least 16")
	})

	t.Run("http transport with bearer token and tls is accepted", func(t *testing.T) {
		tlsCert, tlsKey := writeTempTLS(t)
		env := mergeEnv(baseEnv, map[string]string{
			envTransport:   TransportHTTP,
			envBearerToken: testBearerToken,
			envTLSCert:     tlsCert,
			envTLSKey:      tlsKey,
		})
		setEnv(t, env)
		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, TransportHTTP, cfg.Transport)
		require.Equal(t, testBearerToken, cfg.BearerToken)
		require.True(t, cfg.TLSEnabled())
	})

	t.Run("http transport without tls is rejected", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{
			envTransport:   TransportHTTP,
			envBearerToken: testBearerToken,
		})
		setEnv(t, env)
		_, err := Load()
		require.Error(t, err, "http mode without TLS should require explicit opt-in")
		require.Containsf(t, err.Error(), envAllowInsecureHTTP, "error should name the override: %v", err)
	})

	t.Run("http transport with insecure opt-in is accepted", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{
			envTransport:         TransportHTTP,
			envBearerToken:       testBearerToken,
			envAllowInsecureHTTP: "true",
		})
		setEnv(t, env)
		cfg, err := Load()
		require.NoError(t, err)
		require.True(t, cfg.AllowInsecureHTTP)
		require.False(t, cfg.TLSEnabled())
	})

	t.Run("http transport without bearer mentions the opt-out env", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{envTransport: TransportHTTP})
		setEnv(t, env)
		_, err := Load()
		require.Error(t, err)
		require.Containsf(t, err.Error(), envAllowNoBearer, "error should name the opt-out env: %v", err)
	})

	t.Run("http transport with no-bearer opt-in is accepted without a token", func(t *testing.T) {
		tlsCert, tlsKey := writeTempTLS(t)
		env := mergeEnv(baseEnv, map[string]string{
			envTransport:     TransportHTTP,
			envAllowNoBearer: "true",
			envTLSCert:       tlsCert,
			envTLSKey:        tlsKey,
		})
		setEnv(t, env)
		cfg, err := Load()
		require.NoError(t, err)
		require.True(t, cfg.AllowNoBearer)
		require.Empty(t, cfg.BearerToken)
	})

	t.Run("bearer token and no-bearer opt-in together are rejected", func(t *testing.T) {
		tlsCert, tlsKey := writeTempTLS(t)
		env := mergeEnv(baseEnv, map[string]string{
			envTransport:     TransportHTTP,
			envBearerToken:   testBearerToken,
			envAllowNoBearer: "true",
			envTLSCert:       tlsCert,
			envTLSKey:        tlsKey,
		})
		setEnv(t, env)
		_, err := Load()
		require.Error(t, err)
		require.Containsf(t, err.Error(), envBearerToken, "error should name bearer env: %v", err)
		require.Containsf(t, err.Error(), envAllowNoBearer, "error should name opt-out env: %v", err)
		require.Contains(t, err.Error(), "mutually exclusive")
	})

	t.Run("invalid no-bearer opt-in bool is rejected", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{envAllowNoBearer: "not-a-bool"})
		setEnv(t, env)
		_, err := Load()
		require.Error(t, err)
	})

	t.Run("http transport with only tls cert is rejected", func(t *testing.T) {
		tlsCert := t.TempDir()
		env := mergeEnv(baseEnv, map[string]string{
			envTransport:   TransportHTTP,
			envBearerToken: testBearerToken,
			envTLSCert:     tlsCert,
		})
		setEnv(t, env)
		_, err := Load()
		require.Error(t, err, "expected error when only TLS cert is set")
		require.Containsf(t, err.Error(), envTLSKey, "error should mention missing key env: %v", err)
	})

	t.Run("http transport with only tls key is rejected", func(t *testing.T) {
		tlsKey := t.TempDir()
		env := mergeEnv(baseEnv, map[string]string{
			envTransport:   TransportHTTP,
			envBearerToken: testBearerToken,
			envTLSKey:      tlsKey,
		})
		setEnv(t, env)
		_, err := Load()
		require.Error(t, err, "expected error when only TLS key is set")
		require.Containsf(t, err.Error(), envTLSCert, "error should mention missing cert env: %v", err)
	})

	t.Run("http transport with tls cert as directory is rejected", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{
			envTransport:   TransportHTTP,
			envBearerToken: testBearerToken,
			envTLSCert:     t.TempDir(), // directory, not a regular file
			envTLSKey:      t.TempDir(),
		})
		setEnv(t, env)
		_, err := Load()
		require.Error(t, err, "expected error when TLS path is a directory")
		require.Contains(t, err.Error(), "is not a regular file")
	})

	t.Run("http transport with missing tls cert file is rejected", func(t *testing.T) {
		tlsKey := t.TempDir()
		env := mergeEnv(baseEnv, map[string]string{
			envTransport:   TransportHTTP,
			envBearerToken: testBearerToken,
			envTLSCert:     "/nonexistent/tls.crt",
			envTLSKey:      tlsKey,
		})
		setEnv(t, env)
		_, err := Load()
		require.Error(t, err, "expected error for missing TLS cert file")
	})

	t.Run("unknown transport is rejected", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{envTransport: "websocket"})
		setEnv(t, env)
		_, err := Load()
		require.Error(t, err, "expected error for unknown transport")
		require.Containsf(t, err.Error(), "websocket", "error should name the offending value: %v", err)
	})

	t.Run("max conns override is honored", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{envMaxConns: "25"})
		setEnv(t, env)
		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, int32(25), cfg.MaxConns)
	})

	t.Run("non-positive max conns is rejected", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{envMaxConns: "0"})
		setEnv(t, env)
		_, err := Load()
		require.Error(t, err, "expected error for non-positive max conns")
	})

	t.Run("max conns above the upper bound is rejected", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{envMaxConns: "1000"})
		setEnv(t, env)
		_, err := Load()
		require.Error(t, err, "expected error for max conns > ceiling")
		require.Contains(t, err.Error(), "<= 100")
	})

	t.Run("txn qos override is honored", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{envTxnQoS: "regular"})
		setEnv(t, env)
		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, "regular", cfg.TxnQoS)
	})

	t.Run("txn qos is normalized to lowercase", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{envTxnQoS: "BACKGROUND"})
		setEnv(t, env)
		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, "background", cfg.TxnQoS,
			"CRDB accepts QoS values case-insensitively; config should too")
	})

	t.Run("invalid txn qos is rejected", func(t *testing.T) {
		env := mergeEnv(baseEnv, map[string]string{envTxnQoS: "turbo"})
		setEnv(t, env)
		_, err := Load()
		require.Error(t, err, "expected error for invalid txn qos")
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
		envMaxRowsCount, envTransport, envHTTPListenAddr, envBearerToken,
		envTLSCert, envTLSKey, envAllowInsecureHTTP, envAllowPasswordAuth,
		envLogLevel, envLogPath,
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

func writeTempTLS(t *testing.T) (cert, key string) {
	t.Helper()
	dir := t.TempDir()
	cert = dir + "/tls.crt"
	key = dir + "/tls.key"
	for _, p := range []string{cert, key} {
		require.NoErrorf(t, writeFile(p, "test"), "write %s", p)
	}
	return
}

func writeFile(path, contents string) error {
	return os.WriteFile(path, []byte(contents), 0o600)
}
