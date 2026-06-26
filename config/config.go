// Package config loads server configuration from environment variables.
package config

import (
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/cockroachdb/errors"
)

const (
	envDatabaseURL        = "CRDB_DATABASE_URL"
	envHost               = "CRDB_HOST"
	envPort               = "CRDB_PORT"
	envUser               = "CRDB_USERNAME"
	envPassword           = "CRDB_PWD"
	envSSLMode            = "CRDB_SSL_MODE"
	envCAPath             = "CRDB_SSL_CA_PATH"
	envCertFile           = "CRDB_SSL_CERTFILE"
	envKeyFile            = "CRDB_SSL_KEYFILE"
	envEnableWriteQueries = "CRDB_MCP_ENABLE_WRITE_QUERIES"
	envQueryTimeout       = "CRDB_MCP_QUERY_TIMEOUT"
	envMaxRowsCount       = "CRDB_MCP_MAX_ROWS_COUNT"
	envAllowPasswordAuth  = "CRDB_MCP_ALLOW_PASSWORD_AUTH"
	envTransport          = "CRDB_MCP_TRANSPORT"
	envHTTPListenAddr     = "CRDB_MCP_HTTP_LISTEN_ADDR"
	envBearerToken        = "CRDB_MCP_BEARER_TOKEN"
	envTLSCert            = "CRDB_MCP_TLS_CERT"
	envTLSKey             = "CRDB_MCP_TLS_KEY"
	envAllowInsecureHTTP  = "CRDB_MCP_ALLOW_INSECURE_HTTP"

	defaultPort               = 26257
	defaultSSLMode            = "verify-full"
	defaultQueryTimeout       = 30 * time.Second
	defaultMaxRowsCount int64 = 10000
	defaultTransport          = "stdio"
	defaultHTTPListenAddr     = ":8080"
	minBearerTokenLength      = 16
	bootstrapDB               = "defaultdb"

	// TransportStdio runs the server over stdin/stdout (default).
	TransportStdio = "stdio"
	// TransportHTTP runs the server as an HTTP service with bearer-token auth.
	TransportHTTP = "http"
)

// Config holds the server configuration.
type Config struct {
	DatabaseURL        string
	Host               string
	Port               int
	User               string
	Password           string
	SSLMode            string
	CAPath             string
	CertFile           string
	KeyFile            string
	EnableWriteQueries bool
	QueryTimeout       time.Duration
	MaxRowsCount       int64
	AllowPasswordAuth  bool
	Transport          string
	HTTPListenAddr     string
	BearerToken        string
	TLSCert            string
	TLSKey             string
	AllowInsecureHTTP  bool
}

// Load reads configuration from environment variables.
// Two auth modes are supported, in priority order:
//  1. CRDB_DATABASE_URL - a full libpq connection string (preferred when set).
//  2. Cert-based env vars - CRDB_HOST, CRDB_USERNAME, CRDB_SSL_CERTFILE,
//     CRDB_SSL_KEYFILE, and CRDB_SSL_CA_PATH for verify-ca/verify-full.
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:    os.Getenv(envDatabaseURL),
		Port:           defaultPort,
		QueryTimeout:   defaultQueryTimeout,
		MaxRowsCount:   defaultMaxRowsCount,
		Transport:      defaultTransport,
		HTTPListenAddr: defaultHTTPListenAddr,
		BearerToken:    os.Getenv(envBearerToken),
		TLSCert:        os.Getenv(envTLSCert),
		TLSKey:         os.Getenv(envTLSKey),
	}
	if raw := os.Getenv(envTransport); raw != "" {
		cfg.Transport = raw
	}
	if raw := os.Getenv(envHTTPListenAddr); raw != "" {
		cfg.HTTPListenAddr = raw
	}
	if raw := os.Getenv(envAllowInsecureHTTP); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, errors.Wrapf(err, "%s must be a boolean", envAllowInsecureHTTP)
		}
		cfg.AllowInsecureHTTP = v
	}
	switch cfg.Transport {
	case TransportStdio:
	case TransportHTTP:
		if cfg.BearerToken == "" {
			return nil, errors.Newf("%s is required when %s=%s", envBearerToken, envTransport, TransportHTTP)
		}
		if len(cfg.BearerToken) < minBearerTokenLength {
			return nil, errors.Newf("%s must be at least %d characters", envBearerToken, minBearerTokenLength)
		}
		if err := validateHTTPTLS(cfg); err != nil {
			return nil, err
		}
	default:
		return nil, errors.Newf("%s=%q is not allowed; use %q or %q",
			envTransport, cfg.Transport, TransportStdio, TransportHTTP)
	}

	// CRDB_PORT applies only in cert-based mode; URL mode takes the port from
	// CRDB_DATABASE_URL.
	if raw := os.Getenv(envPort); raw != "" {
		p, err := strconv.Atoi(raw)
		if err != nil {
			return nil, errors.Wrapf(err, "%s must be an integer", envPort)
		}
		cfg.Port = p
	}
	if raw := os.Getenv(envQueryTimeout); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return nil, errors.Wrapf(err, "%s must be a Go duration (e.g. 30s)", envQueryTimeout)
		}
		cfg.QueryTimeout = d
	}
	if raw := os.Getenv(envMaxRowsCount); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return nil, errors.Newf("%s must be a positive integer, got %q", envMaxRowsCount, raw)
		}
		cfg.MaxRowsCount = int64(n)
	}
	if raw := os.Getenv(envEnableWriteQueries); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, errors.Wrapf(err, "%s must be a boolean", envEnableWriteQueries)
		}
		cfg.EnableWriteQueries = v
	}
	if raw := os.Getenv(envAllowPasswordAuth); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, errors.Wrapf(err, "%s must be a boolean", envAllowPasswordAuth)
		}
		cfg.AllowPasswordAuth = v
	}

	if cfg.DatabaseURL != "" {
		if err := validateDatabaseURL(cfg.DatabaseURL); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	return loadCertConfig(cfg)
}

// DSN returns the libpq connection string for the configured auth mode.
func (c *Config) DSN() string {
	if c.DatabaseURL != "" {
		return c.DatabaseURL
	}
	q := url.Values{}
	q.Set("sslmode", c.SSLMode)
	q.Set("sslcert", c.CertFile)
	q.Set("sslkey", c.KeyFile)
	if c.CAPath != "" {
		q.Set("sslrootcert", c.CAPath)
	}
	u := url.URL{
		Scheme:   "postgresql",
		Host:     c.Host + ":" + strconv.Itoa(c.Port),
		Path:     "/" + bootstrapDB,
		RawQuery: q.Encode(),
	}
	if c.Password != "" {
		u.User = url.UserPassword(c.User, c.Password)
	} else {
		u.User = url.User(c.User)
	}
	return u.String()
}

func validateDatabaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return errors.Wrapf(err, "%s is not a valid URL", envDatabaseURL)
	}
	mode := u.Query().Get("sslmode")
	if mode == "" {
		return errors.Newf("%s must include sslmode (require, verify-ca, or verify-full)", envDatabaseURL)
	}
	if !allowedSSLMode(mode) {
		return errors.Newf("%s sslmode=%q is not allowed; use require, verify-ca, or verify-full",
			envDatabaseURL, mode)
	}
	return nil
}

func loadCertConfig(cfg *Config) (*Config, error) {
	cfg.Host = os.Getenv(envHost)
	cfg.User = os.Getenv(envUser)
	cfg.Password = os.Getenv(envPassword)
	cfg.SSLMode = os.Getenv(envSSLMode)
	cfg.CAPath = os.Getenv(envCAPath)
	cfg.CertFile = os.Getenv(envCertFile)
	cfg.KeyFile = os.Getenv(envKeyFile)

	if cfg.SSLMode == "" {
		cfg.SSLMode = defaultSSLMode
	}
	if !allowedSSLMode(cfg.SSLMode) {
		return nil, errors.Newf("%s=%q is not allowed; use require, verify-ca, or verify-full",
			envSSLMode, cfg.SSLMode)
	}
	var missing []string
	if cfg.Host == "" {
		missing = append(missing, envHost)
	}
	if cfg.User == "" {
		missing = append(missing, envUser)
	}
	if cfg.CertFile == "" {
		missing = append(missing, envCertFile)
	}
	if cfg.KeyFile == "" {
		missing = append(missing, envKeyFile)
	}
	if cfg.SSLMode == "verify-ca" || cfg.SSLMode == "verify-full" {
		if cfg.CAPath == "" {
			missing = append(missing, envCAPath)
		}
	}
	if len(missing) > 0 {
		return nil, errors.Newf("set %s or provide cert env vars; missing: %v",
			envDatabaseURL, missing)
	}

	for label, path := range map[string]string{
		envCertFile: cfg.CertFile,
		envKeyFile:  cfg.KeyFile,
		envCAPath:   cfg.CAPath,
	} {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			return nil, errors.Wrapf(err, "%s: cert file not accessible", label)
		}
	}
	return cfg, nil
}

// TLSEnabled reports whether the HTTP transport should be served over TLS.
func (c *Config) TLSEnabled() bool {
	return c.TLSCert != "" && c.TLSKey != ""
}

// validateHTTPTLS enforces the SECSERV-422 default-secure policy: HTTP mode
// must serve TLS unless the operator explicitly opts into cleartext via
// CRDB_MCP_ALLOW_INSECURE_HTTP=true. Cert and key are required together, and
// any path provided must exist on disk.
func validateHTTPTLS(cfg *Config) error {
	switch {
	case cfg.TLSCert != "" && cfg.TLSKey == "":
		return errors.Newf("%s is set but %s is empty; both are required for TLS", envTLSCert, envTLSKey)
	case cfg.TLSKey != "" && cfg.TLSCert == "":
		return errors.Newf("%s is set but %s is empty; both are required for TLS", envTLSKey, envTLSCert)
	case cfg.TLSCert == "" && cfg.TLSKey == "" && !cfg.AllowInsecureHTTP:
		return errors.Newf(
			"HTTP transport requires TLS: set %s and %s, or explicitly opt into cleartext with %s=true",
			envTLSCert, envTLSKey, envAllowInsecureHTTP)
	}
	for label, path := range map[string]string{
		envTLSCert: cfg.TLSCert,
		envTLSKey:  cfg.TLSKey,
	} {
		if path == "" {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			return errors.Wrapf(err, "%s: TLS file not accessible", label)
		}
		if !info.Mode().IsRegular() {
			return errors.Newf("%s=%q is not a regular file", label, path)
		}
	}
	return nil
}

func allowedSSLMode(mode string) bool {
	switch mode {
	case "require", "verify-ca", "verify-full":
		return true
	}
	return false
}
