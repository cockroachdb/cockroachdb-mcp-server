// Package config loads server configuration from environment variables.
package config

import (
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"go.uber.org/zap/zapcore"
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
	envAllowNoBearer      = "CRDB_MCP_ALLOW_NO_BEARER"
	envMaxConns           = "CRDB_MCP_MAX_CONNS"
	envTxnQoS             = "CRDB_MCP_TXN_QOS"
	envLogLevel           = "CRDB_MCP_LOG_LEVEL"
	envLogPath            = "CRDB_MCP_LOG_PATH"
	envHTTPRPS            = "CRDB_MCP_HTTP_RPS"
	envHTTPBurst          = "CRDB_MCP_HTTP_BURST"
	envHTTPMaxConcurrent  = "CRDB_MCP_HTTP_MAX_CONCURRENT"
	envHTTPTrustXFF       = "CRDB_MCP_HTTP_TRUST_XFF"

	defaultPort               = 26257
	defaultSSLMode            = "verify-full"
	defaultQueryTimeout       = 30 * time.Second
	defaultMaxRowsCount int64 = 10000
	defaultLogLevel           = zapcore.InfoLevel
	defaultMaxConns     int32 = 10
	// maxAllowedConns caps user-supplied CRDB_MCP_MAX_CONNS.
	maxAllowedConns       int32 = 100
	defaultTransport            = "stdio"
	defaultHTTPListenAddr       = ":8080"
	minBearerTokenLength        = 16
	bootstrapDB                 = "defaultdb"

	// TransportStdio runs the server over stdin/stdout (default).
	TransportStdio = "stdio"
	// TransportHTTP runs the server as an HTTP service with bearer-token auth.
	TransportHTTP = "http"
)

var allowedQoSValues = []string{"background", "regular", "critical"}

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
	LogLevel           zapcore.Level
	LogPath            string
	MaxConns           int32
	// TxnQoS is set only when CRDB_MCP_TXN_QOS is explicit. Empty means
	// "let the adapter decide" - the adapter respects a DSN-supplied value
	// if present, otherwise falls back to its background default.
	TxnQoS            string
	AllowPasswordAuth bool
	Transport         string
	HTTPListenAddr    string
	BearerToken       string
	TLSCert           string
	TLSKey            string
	AllowInsecureHTTP bool
	// AllowNoBearer lets HTTP mode start without a bearer token. Auth must
	// then be provided upstream (reverse proxy, gateway, mTLS).
	AllowNoBearer bool
	// HTTPRPS is the per-client request-rate limit for HTTP mode. 0 disables.
	HTTPRPS float64
	// HTTPBurst is the per-client burst; defaults to 2*HTTPRPS when unset.
	HTTPBurst int
	// HTTPMaxConcurrent caps in-flight HTTP requests across all clients. 0 disables.
	HTTPMaxConcurrent int
	// HTTPTrustXFF keys the rate limiter by the rightmost X-Forwarded-For
	// entry for unauthenticated requests. Only safe behind a trusted proxy
	// that appends to XFF; off by default because the header is client-forgeable.
	HTTPTrustXFF bool
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
		MaxConns:       defaultMaxConns,
		Transport:      defaultTransport,
		HTTPListenAddr: defaultHTTPListenAddr,
		BearerToken:    os.Getenv(envBearerToken),
		TLSCert:        os.Getenv(envTLSCert),
		TLSKey:         os.Getenv(envTLSKey),
		LogLevel:       defaultLogLevel,
		LogPath:        os.Getenv(envLogPath),
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
	if raw := os.Getenv(envAllowNoBearer); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, errors.Wrapf(err, "%s must be a boolean", envAllowNoBearer)
		}
		cfg.AllowNoBearer = v
	}
	switch cfg.Transport {
	case TransportStdio:
	case TransportHTTP:
		if cfg.BearerToken == "" && !cfg.AllowNoBearer {
			return nil, errors.Newf("%s is required when %s=%s; set %s=true to opt out and provide auth upstream",
				envBearerToken, envTransport, TransportHTTP, envAllowNoBearer)
		}
		if cfg.BearerToken != "" && cfg.AllowNoBearer {
			return nil, errors.Newf("%s and %s are mutually exclusive; unset one",
				envBearerToken, envAllowNoBearer)
		}
		if cfg.BearerToken != "" && len(cfg.BearerToken) < minBearerTokenLength {
			return nil, errors.Newf("%s must be at least %d characters", envBearerToken, minBearerTokenLength)
		}
		if err := validateHTTPTLS(cfg); err != nil {
			return nil, err
		}
	default:
		return nil, errors.Newf("%s=%q is not allowed; use %q or %q",
			envTransport, cfg.Transport, TransportStdio, TransportHTTP)
	}
	if raw := os.Getenv(envLogLevel); raw != "" {
		level, err := ParseLogLevel(raw)
		if err != nil {
			return nil, err
		}
		cfg.LogLevel = level
	}
	if cfg.LogPath != "" && cfg.LogPath != "-" {
		f, err := os.OpenFile(cfg.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, errors.Wrapf(err, "%s=%q is not writable", envLogPath, cfg.LogPath)
		}
		_ = f.Close()
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
	if raw := os.Getenv(envMaxConns); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || n <= 0 {
			return nil, errors.Newf("%s must be a positive integer, got %q", envMaxConns, raw)
		}
		if int32(n) > maxAllowedConns {
			return nil, errors.Newf("%s must be <= %d, got %d", envMaxConns, maxAllowedConns, n)
		}
		cfg.MaxConns = int32(n)
	}
	if raw := os.Getenv(envTxnQoS); raw != "" {
		v := strings.ToLower(raw)
		if !slices.Contains(allowedQoSValues, v) {
			return nil, errors.Newf("%s=%q is not allowed; use %s",
				envTxnQoS, raw, strings.Join(allowedQoSValues, ", "))
		}
		cfg.TxnQoS = v
	}
	if raw := os.Getenv(envHTTPRPS); raw != "" {
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || v < 0 {
			return nil, errors.Newf("%s must be a non-negative number, got %q", envHTTPRPS, raw)
		}
		cfg.HTTPRPS = v
	}
	if raw := os.Getenv(envHTTPBurst); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 0 {
			return nil, errors.Newf("%s must be a non-negative integer, got %q", envHTTPBurst, raw)
		}
		cfg.HTTPBurst = v
	}
	if raw := os.Getenv(envHTTPMaxConcurrent); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 0 {
			return nil, errors.Newf("%s must be a non-negative integer, got %q", envHTTPMaxConcurrent, raw)
		}
		cfg.HTTPMaxConcurrent = v
	}
	if raw := os.Getenv(envHTTPTrustXFF); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, errors.Wrapf(err, "%s must be a boolean", envHTTPTrustXFF)
		}
		cfg.HTTPTrustXFF = v
	}
	if cfg.HTTPBurst > 0 && cfg.HTTPRPS <= 0 {
		return nil, errors.Newf("%s requires %s > 0", envHTTPBurst, envHTTPRPS)
	}

	if cfg.DatabaseURL != "" {
		if err := validateDatabaseURL(cfg.DatabaseURL); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	return loadCertConfig(cfg)
}

// ParseLogLevel converts a textual log level to zapcore.Level. Accepts debug,
// info, warn, error (case-insensitive).
func ParseLogLevel(s string) (zapcore.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	default:
		return 0, errors.Newf("%s=%q is not allowed; use debug, info, warn, or error",
			envLogLevel, s)
	}
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
