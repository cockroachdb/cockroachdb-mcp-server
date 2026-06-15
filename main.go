// cockroachdb-mcp-server is the Model Context Protocol server for CockroachDB.
package main

import (
	"context"
	stderrors "errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/cockroachdb/cockroachdb-mcp-server/auth"
	"github.com/cockroachdb/cockroachdb-mcp-server/config"
	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/cockroachdb/cockroachdb-mcp-server/tools"
	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/sync/errgroup"
)

const (
	serverName        = "cockroachdb-mcp-server"
	shutdownPeriod    = 10 * time.Second
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	idleTimeout       = 120 * time.Second
	maxHeaderBytes    = 32 << 10 // 32 KiB
)

var serverVersion = "0.1.0"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("%s %s\n", serverName, serverVersion)
		return
	}

	if err := run(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return errors.Wrap(err, "load config")
	}

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
	tools.NewToolHandlers(dm, cfg).RegisterTools(server)

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
	log.Printf("Starting %s %s on stdio", serverName, serverVersion)
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
	var err error
	if cfg.TLSEnabled() {
		log.Printf("Starting %s %s on https %s", serverName, serverVersion, cfg.HTTPListenAddr)
		err = httpServer.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey)
	} else {
		log.Printf("WARNING: %s running on http %s without TLS (CRDB_MCP_ALLOW_INSECURE_HTTP=true). "+
			"Bearer tokens travel in cleartext; terminate TLS at a trusted reverse proxy.",
			serverName, cfg.HTTPListenAddr)
		err = httpServer.ListenAndServe()
	}
	if err != nil && !stderrors.Is(err, http.ErrServerClosed) {
		return errors.Wrap(err, "http server")
	}
	return nil
}

// newHTTPMux returns the top-level HTTP handler: /healthz and /ready are
// unauthenticated so orchestrators can probe without holding the bearer
// token; every other path goes through the bearer middleware.
func newHTTPMux(bearerToken string, mcpHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", health)
	mux.HandleFunc("/ready", health)
	mux.Handle("/", auth.Bearer(bearerToken, mcpHandler))
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
