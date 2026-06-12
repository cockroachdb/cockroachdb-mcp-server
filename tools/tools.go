// Package tools implements the MCP tools served by the cockroachdb-mcp-server.
package tools

import (
	"context"

	"github.com/cockroachdb/cockroachdb-mcp-server/config"
	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DBManager is the narrow database surface that tool handlers depend on. The
// real *db.Manager satisfies it; tests use a fake.
type DBManager interface {
	Query(ctx context.Context, sql string) (*db.QueryResult, error)
}

// ToolHandlers wires MCP tool handlers to the underlying database querier and
// server configuration.
type ToolHandlers struct {
	dm  DBManager
	cfg *config.Config
}

// NewToolHandlers constructs a handler bundle bound to the given database and
// server configuration.
func NewToolHandlers(dm DBManager, cfg *config.Config) *ToolHandlers {
	return &ToolHandlers{dm: dm, cfg: cfg}
}

// RegisterTools registers all MCP tools on the server. mcp.AddTool derives
// each tool's schema from its typed params struct (jsonschema tags) and
// validates input upstream.
func (h *ToolHandlers) RegisterTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_databases",
		Description: "List all databases in the CockroachDB cluster.",
	}, h.listDatabases)
}
