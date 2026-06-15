package tools

import (
	"context"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// showStatement handles the show_statement tool. Parses the agent-supplied
// SQL, requires it to be a SHOW, and appends LIMIT/OFFSET capped at
// MaxRowsCount.
func (h *ToolHandlers) showStatement(
	ctx context.Context, _ *mcp.CallToolRequest, params ShowStatementParams,
) (*mcp.CallToolResult, any, error) {
	stmt, err := parseSingleStatement(params.Query)
	if err != nil {
		return nil, nil, err
	}
	if err := validateShowStatement(stmt); err != nil {
		return nil, nil, err
	}

	wrapped, err := db.SafeFormat("SELECT * FROM [%1]", db.SQL(stmt.String()))
	if err != nil {
		return nil, nil, err
	}
	query, err := h.applyLimitOffset(wrapped, params.Limit, params.Offset)
	if err != nil {
		return nil, nil, err
	}

	res, err := h.dm.Query(ctx, query)
	if err != nil {
		return nil, nil, errors.Wrap(err, "show statement")
	}
	result, err := queryResultToMCP(res)
	return result, nil, err
}
