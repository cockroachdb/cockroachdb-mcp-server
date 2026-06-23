package tools

import (
	"context"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/cockroachdb/cockroachdb-parser/pkg/sql/sem/tree"
	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// explainQuery handles the explain_query tool.
func (h *ToolHandlers) explainQuery(
	ctx context.Context, _ *mcp.CallToolRequest, params ExplainQueryParams,
) (*mcp.CallToolResult, any, error) {
	stmt, err := parseSingleStatement(params.Query)
	if err != nil {
		return nil, nil, err
	}

	var query string
	switch stmt.(type) {
	case *tree.ExplainAnalyze:
		return nil, nil, errors.New("EXPLAIN ANALYZE is not allowed; it executes the inner statement")
	case *tree.Explain:
		query = stmt.String()
	default:
		query, err = db.SafeFormat("EXPLAIN %1", db.SQL(stmt.String()))
		if err != nil {
			return nil, nil, err
		}
	}

	res, err := h.dm.Query(ctx, query)
	if err != nil {
		return nil, nil, errors.Wrap(err, "explain query")
	}
	result, err := queryResultToMCP(res)
	return result, nil, err
}
