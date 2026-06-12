package tools

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const showRunningQueriesQuery = `
SELECT query_id, node_id, user_name, start, query, client_address, application_name, phase
FROM [SHOW CLUSTER STATEMENTS]
ORDER BY start DESC`

// showRunningQueries handles the show_running_queries tool.
func (h *ToolHandlers) showRunningQueries(
	ctx context.Context, _ *mcp.CallToolRequest, params ShowRunningQueriesParams,
) (*mcp.CallToolResult, any, error) {
	sql, err := h.applyLimitOffset(showRunningQueriesQuery, params.Limit, params.Offset)
	if err != nil {
		return nil, nil, err
	}
	res, err := h.dm.Query(ctx, sql)
	if err != nil {
		return nil, nil, errors.Wrap(err, "show running queries")
	}
	result, err := queryResultToMCP(res)
	return result, nil, err
}
