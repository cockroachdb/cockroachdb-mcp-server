package tools

import (
	"context"

	"github.com/cockroachdb/cockroachdb-parser/pkg/sql/sem/tree"
	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// updateRows handles the update_rows tool.
func (h *ToolHandlers) updateRows(
	ctx context.Context, _ *mcp.CallToolRequest, params UpdateRowsParams,
) (*mcp.CallToolResult, any, error) {
	stmt, err := parseSingleStatement(params.Statement)
	if err != nil {
		return nil, nil, err
	}
	upd, ok := stmt.(*tree.Update)
	if !ok {
		return nil, nil, errors.Newf("expected an UPDATE statement, got %s", stmt.StatementTag())
	}
	if upd.Where == nil {
		return nil, nil, errors.New(
			"UPDATE without a WHERE clause is not allowed; add a WHERE clause selecting only the intended rows")
	}
	// A nested write in a CTE, FROM source, or subquery is rejected; reads are fine.
	// Otherwise, they would run as side effects this tool neither reports nor WHERE-guards.
	if err := ensureNoHiddenDML(upd); err != nil {
		return nil, nil, errors.Wrap(err, "UPDATE")
	}
	return h.runMutation(ctx, stmt, "update rows")
}
