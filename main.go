// cockroachdb-mcp-server is the Model Context Protocol server for CockroachDB.
package main

import (
	"context"
	"crypto/tls"
	stderrors "errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cockroachdb/cockroachdb-mcp-server/auth"
	"github.com/cockroachdb/cockroachdb-mcp-server/config"
	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/cockroachdb/cockroachdb-mcp-server/logging"
	"github.com/cockroachdb/cockroachdb-mcp-server/middleware"
	"github.com/cockroachdb/cockroachdb-mcp-server/tools"
	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"
)

const (
	serverName         = "cockroachdb-mcp-server"
	shutdownPeriod     = 10 * time.Second
	readHeaderTimeout  = 10 * time.Second
	readTimeout        = 30 * time.Second
	idleTimeout        = 120 * time.Second
	maxHeaderBytes     = 32 << 10 // 32 KiB
	healthCheckTimeout = 3 * time.Second
)

var serverVersion = "0.1.0"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	healthCheck := flag.Bool("healthcheck", false, "probe local /healthz and exit 0/1; intended for Docker HEALTHCHECK in HTTP mode")
	flag.Parse()
	if *showVersion {
		fmt.Printf("%s %s\n", serverName, serverVersion)
		return
	}
	if *healthCheck {
		if !healthCheckEnabled() {
			return
		}
		if err := probeHealthz(healthCheckURL()); err != nil {
			fmt.Fprintf(os.Stderr, "healthcheck failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Bootstrap logger keeps config-load fatals visible. Stderr-only so a
	// bad CRDB_MCP_LOG_PATH can't swallow its own error.
	bootstrap, err := logging.NewLogger(zapcore.InfoLevel, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: build bootstrap logger: %v\n", err)
		os.Exit(1)
	}
	zap.ReplaceGlobals(bootstrap)

	if err := run(); err != nil {
		zap.L().Error("fatal", zap.Error(err))
		_ = zap.L().Sync()
		os.Exit(1)
	}
}

// healthCheckEnabled reports whether the self-probe applies: /healthz only
// exists in HTTP mode, so stdio containers pass trivially.
func healthCheckEnabled() bool {
	return strings.EqualFold(os.Getenv("CRDB_MCP_TRANSPORT"), config.TransportHTTP)
}

// healthCheckURL derives the probe URL from the server's own listen env vars,
// rewriting wildcard hosts to loopback.
func healthCheckURL() string {
	addr := os.Getenv("CRDB_MCP_HTTP_LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	if host, port, err := net.SplitHostPort(addr); err == nil {
		switch host {
		case "", "0.0.0.0", "::":
			host = "127.0.0.1"
		}
		addr = net.JoinHostPort(host, port)
	}
	scheme := "http"
	if os.Getenv("CRDB_MCP_TLS_CERT") != "" && os.Getenv("CRDB_MCP_TLS_KEY") != "" {
		scheme = "https"
	}
	return scheme + "://" + addr + "/healthz"
}

// probeHealthz issues a single GET and returns nil only on 200. TLS
// verification is skipped: this is a loopback self-check.
func probeHealthz(url string) error {
	client := &http.Client{
		Timeout: healthCheckTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // loopback self-probe
		},
	}
	resp, err := client.Get(url)
	if err != nil {
		return errors.Wrap(err, "probe healthz")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return errors.Newf("/healthz returned %d", resp.StatusCode)
	}
	return nil
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return errors.Wrap(err, "load config")
	}

	logger, err := logging.NewLogger(cfg.LogLevel, cfg.LogPath)
	if err != nil {
		return errors.Wrapf(err, "open log path %q", cfg.LogPath)
	}
	zap.ReplaceGlobals(logger)
	defer func() { _ = logger.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dm, err := db.NewManager(ctx, cfg)
	if err != nil {
		return errors.Wrap(err, "initialize database manager")
	}
	defer dm.Close()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, nil)
	server.AddReceivingMiddleware(middleware.ToolCallLogger)
	tools.NewToolHandlers(dm, cfg).RegisterTools(server)

	zap.L().Info("starting server",
		zap.String("name", serverName),
		zap.String("version", serverVersion),
		zap.String("transport", cfg.Transport))

	switch cfg.Transport {
	case config.TransportStdio:
		return runStdio(ctx, server)
	case config.TransportHTTP:
		return runHTTP(ctx, server, cfg)
	default:
		return errors.Newf("unsupported transport %q", cfg.Transport)
	}
}

func runStdio(ctx context.Context, server *mcp.Server) error {
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil &&
		!stderrors.Is(err, context.Canceled) && !stderrors.Is(err, io.EOF) {
		return errors.Wrap(err, "server")
	}
	return nil
}

func runHTTP(ctx context.Context, server *mcp.Server, cfg *config.Config) error {
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, nil)

	httpServer := &http.Server{
		Addr:              cfg.HTTPListenAddr,
		Handler:           newHTTPMux(cfg.BearerToken, mcpHandler),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		<-gctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownPeriod)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return errors.Wrap(err, "shutdown http server")
		}
		return nil
	})
	g.Go(func() error {
		return serveHTTP(httpServer, cfg)
	})
	return g.Wait()
}

func serveHTTP(httpServer *http.Server, cfg *config.Config) error {
	if cfg.AllowNoBearer {
		zap.L().Warn("bearer-token enforcement disabled; ensure auth is provided upstream (reverse proxy, gateway, k8s NetworkPolicy, etc)",
			zap.String("name", serverName),
			zap.String("opt_out_env", "CRDB_MCP_ALLOW_NO_BEARER"))
	}
	var err error
	if cfg.TLSEnabled() {
		zap.L().Info("http transport listening",
			zap.String("name", serverName),
			zap.String("addr", cfg.HTTPListenAddr),
			zap.Bool("tls", true))
		err = httpServer.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey)
	} else {
		zap.L().Warn("http transport listening without TLS; bearer tokens travel cleartext",
			zap.String("name", serverName),
			zap.String("addr", cfg.HTTPListenAddr))
		err = httpServer.ListenAndServe()
	}
	if err != nil && !stderrors.Is(err, http.ErrServerClosed) {
		return errors.Wrap(err, "http server")
	}
	return nil
}

// newHTTPMux returns the top-level HTTP handler: /healthz and /ready are
// unauthenticated for orchestrator probes; other paths go through the bearer
// middleware unless the token is empty (CRDB_MCP_ALLOW_NO_BEARER=true).
func newHTTPMux(bearerToken string, mcpHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", health)
	mux.HandleFunc("/ready", health)
	if bearerToken == "" {
		mux.Handle("/", mcpHandler)
	} else {
		mux.Handle("/", auth.Bearer(bearerToken, mcpHandler))
	}
	return mux
}

func health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
