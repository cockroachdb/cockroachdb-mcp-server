package tools

import (
	"context"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// listTables handles the list_tables tool.
func (h *ToolHandlers) listTables(
	ctx context.Context, _ *mcp.CallToolRequest, params ListTablesParams,
) (*mcp.CallToolResult, any, error) {
	if params.Database == "" {
		return nil, nil, errors.New("database is required")
	}
	query, err := db.SafeFormat(
		"SELECT * FROM [SHOW TABLES FROM %1]",
		db.Identifier(params.Database))
	if err != nil {
		return nil, nil, err
	}
	sql, err := h.applyLimitOffset(query, params.Limit, params.Offset, defaultListTablesLimit)
	if err != nil {
		return nil, nil, err
	}
	res, err := h.dm.Query(ctx, sql)
	if err != nil {
		return nil, nil, errors.Wrap(err, "list tables")
	}
	result, err := queryResultToMCP(res)
	return result, nil, err
}
