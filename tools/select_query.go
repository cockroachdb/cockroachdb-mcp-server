package tools

import (
	"context"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// selectQuery handles the select_query tool. Parses the agent-supplied SQL,
// requires it to be a single SELECT, and appends a default LIMIT capped at
// MaxRowsCount when the caller does not specify one.
func (h *ToolHandlers) selectQuery(
	ctx context.Context, _ *mcp.CallToolRequest, params SelectQueryParams,
) (*mcp.CallToolResult, any, error) {
	stmt, err := parseSingleStatement(params.Query)
	if err != nil {
		return nil, nil, err
	}
	sel, err := ensureSelectOnly(stmt)
	if err != nil {
		return nil, nil, err
	}
	if err := validateLimitClause(sel.Limit, h.cfg.MaxRowsCount); err != nil {
		return nil, nil, err
	}

	query := sel.String()
	if sel.Limit == nil || sel.Limit.Count == nil {
		limit := min(defaultRowLimit, h.cfg.MaxRowsCount)
		query, err = db.SafeFormat("%1 LIMIT %2", db.SQL(query), limit)
		if err != nil {
			return nil, nil, err
		}
	}

	res, err := h.dm.Query(ctx, query)
	if err != nil {
		return nil, nil, errors.Wrap(err, "select query")
	}
	result, err := queryResultToMCP(res)
	return result, nil, err
}
