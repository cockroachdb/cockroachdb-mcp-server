// cockroachdb-mcp-server is the Model Context Protocol server for CockroachDB.
package main

import (
	"context"
	stderrors "errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os/signal"
	"syscall"

	"github.com/cockroachdb/cockroachdb-mcp-server/config"
	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/cockroachdb/cockroachdb-mcp-server/tools"
	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const serverName = "cockroachdb-mcp-server"

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

	log.Printf("Starting %s %s on stdio", serverName, serverVersion)
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil &&
		!stderrors.Is(err, context.Canceled) && !stderrors.Is(err, io.EOF) {
		return errors.Wrap(err, "server")
	}
	return nil
}
