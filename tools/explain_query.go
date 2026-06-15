package tools

import (
	"context"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// explainQuery handles the explain_query tool. Parses the agent-supplied SQL,
// requires it to be a single SELECT (recursively, so DML cannot hide in CTEs
// or bracket subqueries), then prefixes EXPLAIN before execution.
func (h *ToolHandlers) explainQuery(
	ctx context.Context, _ *mcp.CallToolRequest, params ExplainQueryParams,
) (*mcp.CallToolResult, any, error) {
	stmt, err := parseSingleStatement(params.Query)
	if err != nil {
		return nil, nil, err
	}
	if _, err := ensureSelectOnly(stmt); err != nil {
		return nil, nil, err
	}

	query, err := db.SafeFormat("EXPLAIN %1", db.SQL(stmt.String()))
	if err != nil {
		return nil, nil, err
	}
	res, err := h.dm.Query(ctx, query)
	if err != nil {
		return nil, nil, errors.Wrap(err, "explain query")
	}
	result, err := queryResultToMCP(res)
	return result, nil, err
}
