package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cockroachdb/cockroachdb-mcp-server/internal/config"
	"github.com/cockroachdb/cockroachdb-mcp-server/internal/tools/dev"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run() error {
	ctx := context.Background()

	// Load configuration
	_, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create MCP server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "cockroachdb-mcp-server",
		Version: "1.0.0",
	}, nil)

	// Register list_databases tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_databases",
		Description: "List all databases in the CockroachDB cluster",
	}, dev.ListDatabases)

	// Start server in STDIO mode
	return server.Run(ctx, &mcp.StdioTransport{})
}
