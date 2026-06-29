package tools

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// listSQLUsers handles the list_sql_users tool.
func (h *ToolHandlers) listSQLUsers(
	ctx context.Context, _ *mcp.CallToolRequest, params ListSQLUsersParams,
) (*mcp.CallToolResult, any, error) {
	const query = `SELECT * FROM [SHOW USERS]`
	sql, err := h.applyLimitOffset(query, params.Limit, params.Offset)
	if err != nil {
		return nil, nil, err
	}
	res, err := h.dm.Query(ctx, sql)
	if err != nil {
		return nil, nil, errors.Wrap(err, "list sql users")
	}
	result, err := queryResultToMCP(res)
	return result, nil, err
}
