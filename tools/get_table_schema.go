package tools

import (
	"context"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// getTableSchema handles the get_table_schema tool.
func (h *ToolHandlers) getTableSchema(
	ctx context.Context, _ *mcp.CallToolRequest, params TableSchemaParams,
) (*mcp.CallToolResult, any, error) {
	if params.Database == "" {
		return nil, nil, errors.New("database is required")
	}
	if params.Table == "" {
		return nil, nil, errors.New("table is required")
	}
	schema := params.Schema
	if schema == "" {
		schema = "public"
	}
	sql, err := db.SafeFormat("SHOW CREATE TABLE %1",
		db.QualifiedIdentifier{params.Database, schema, params.Table})
	if err != nil {
		return nil, nil, err
	}
	res, err := h.dm.Query(ctx, sql)
	if err != nil {
		return nil, nil, errors.Wrap(err, "get table schema")
	}
	result, err := queryResultToMCP(res)
	return result, nil, err
}
