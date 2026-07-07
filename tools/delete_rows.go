package tools

import (
	"context"

	"github.com/cockroachdb/cockroachdb-parser/pkg/sql/sem/tree"
	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// deleteRows handles the delete_rows tool.
func (h *ToolHandlers) deleteRows(
	ctx context.Context, _ *mcp.CallToolRequest, params DeleteRowsParams,
) (*mcp.CallToolResult, any, error) {
	stmt, err := parseSingleStatement(params.Statement)
	if err != nil {
		return nil, nil, err
	}
	del, ok := stmt.(*tree.Delete)
	if !ok {
		return nil, nil, errors.Newf("expected a DELETE statement, got %s", stmt.StatementTag())
	}
	if del.Where == nil {
		return nil, nil, errors.New(
			"DELETE without a WHERE clause is not allowed; add a WHERE clause selecting only the intended rows")
	}
	// A nested write in a CTE, USING source, or subquery is rejected; reads are fine.
	// Otherwise, they would run as side effects this tool neither reports nor WHERE-guards.
	if err := ensureNoHiddenDML(del); err != nil {
		return nil, nil, errors.Wrap(err, "DELETE")
	}
	return h.runMutation(ctx, stmt, "delete rows")
}
