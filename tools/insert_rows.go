package tools

import (
	"context"

	"github.com/cockroachdb/cockroachdb-parser/pkg/sql/sem/tree"
	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// insertRows handles the insert_rows tool.
func (h *ToolHandlers) insertRows(
	ctx context.Context, _ *mcp.CallToolRequest, params InsertRowsParams,
) (*mcp.CallToolResult, any, error) {
	stmt, err := parseSingleStatement(params.Statement)
	if err != nil {
		return nil, nil, err
	}
	ins, ok := stmt.(*tree.Insert)
	if !ok {
		return nil, nil, errors.Newf("expected an INSERT statement, got %s", stmt.StatementTag())
	}
	// pgxpool.Exec drops RETURNING rows; reject so the agent uses select_query.
	if tree.HasReturningClause(ins.Returning) {
		return nil, nil, errors.New("INSERT ... RETURNING is not supported; use select_query for reads")
	}
	// Block DML hidden inside statement-level CTEs (WITH d AS (DELETE ...) INSERT ...).
	if ins.With != nil {
		for _, cte := range ins.With.CTEList {
			if _, err := ensureSelectOnly(cte.Stmt); err != nil {
				return nil, nil, errors.Wrap(err, "INSERT CTE contains a non-SELECT statement")
			}
		}
	}
	// Block DML hidden inside INSERT ... SELECT (CTEs, bracket subqueries).
	// VALUES clauses fall through ensureSelectOnly as a no-op.
	if ins.Rows != nil {
		if _, err := ensureSelectOnly(ins.Rows); err != nil {
			return nil, nil, errors.Wrap(err, "INSERT source")
		}
	}

	rows, err := h.dm.Exec(ctx, stmt.String())
	if err != nil {
		return nil, nil, errors.Wrap(err, "insert rows")
	}
	return writeOK(map[string]any{
		"table":         tree.AsString(ins.Table),
		"rows_affected": rows,
	})
}
