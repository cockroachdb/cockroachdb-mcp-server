package tools

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// listDatabases handles the list_databases tool.
func (h *ToolHandlers) listDatabases(
	ctx context.Context, _ *mcp.CallToolRequest, params ListDatabasesParams,
) (*mcp.CallToolResult, any, error) {
	const query = `SELECT * FROM [SHOW DATABASES]`
	sql, err := h.applyLimitOffset(query, params.Limit, params.Offset, defaultListDatabasesLimit)
	if err != nil {
		return nil, nil, err
	}
	res, err := h.dm.Query(ctx, sql)
	if err != nil {
		return nil, nil, errors.Wrap(err, "list databases")
	}
	result, err := queryResultToMCP(res)
	return result, nil, err
}
