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
	Exec(ctx context.Context, sql string) (int64, error)
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
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: new(false)}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_databases",
		Description: "List all databases in the CockroachDB cluster.",
		Annotations: readOnly,
	}, h.listDatabases)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_tables",
		Description: "List all tables in a database.",
		Annotations: readOnly,
	}, h.listTables)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_table_schema",
		Description: "Get detailed schema information for a table including columns and indexes.",
		Annotations: readOnly,
	}, h.getTableSchema)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "show_running_queries",
		Description: "List in-flight SQL statements on the CockroachDB cluster.",
		Annotations: readOnly,
	}, h.showRunningQueries)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_cluster",
		Description: "Return CockroachDB cluster identity and version metadata. Restricted fields are null with the reason in 'unavailable'.",
		Annotations: readOnly,
	}, h.getCluster)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_sql_users",
		Description: "List SQL users defined in the CockroachDB cluster.",
		Annotations: readOnly,
	}, h.listSQLUsers)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_cluster_nodes",
		Description: "List CockroachDB cluster nodes with address, liveness, and locality. Requires admin or VIEWCLUSTERMETADATA.",
		Annotations: readOnly,
	}, h.listClusterNodes)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "select_query",
		Description: "Execute a single SELECT statement.",
		Annotations: readOnly,
	}, h.selectQuery)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "explain_query",
		Description: "Return the EXPLAIN plan for a single SQL statement.",
		Annotations: readOnly,
	}, h.explainQuery)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "show_statement",
		Description: "Execute a single SHOW statement.",
		Annotations: readOnly,
	}, h.showStatement)

	// Write tools are only enabled when CRDB_MCP_ENABLE_WRITE_QUERIES=true.
	if h.cfg.EnableWriteQueries {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "create_database",
			Description: "Create a database.",
			Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false), OpenWorldHint: new(false)},
		}, h.createDatabase)

		mcp.AddTool(server, &mcp.Tool{
			Name:        "create_table",
			Description: "Execute a single CREATE TABLE statement.",
			Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false), OpenWorldHint: new(false)},
		}, h.createTable)

		mcp.AddTool(server, &mcp.Tool{
			Name:        "insert_rows",
			Description: "Execute a single INSERT statement. Returns the number of rows affected.",
			Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true), OpenWorldHint: new(false)},
		}, h.insertRows)

		mcp.AddTool(server, &mcp.Tool{
			Name:        "update_rows",
			Description: "Execute a single UPDATE statement with a mandatory WHERE clause. Returns the number of rows affected.",
			Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true), OpenWorldHint: new(false)},
		}, h.updateRows)

		mcp.AddTool(server, &mcp.Tool{
			Name:        "delete_rows",
			Description: "Execute a single DELETE statement with a mandatory WHERE clause. Returns the number of rows affected.",
			Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true), IdempotentHint: true, OpenWorldHint: new(false)},
		}, h.deleteRows)
	}
}
