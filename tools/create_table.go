package tools

import (
	"context"

	"github.com/cockroachdb/cockroachdb-parser/pkg/sql/sem/tree"
	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// createTable handles the create_table tool.
func (h *ToolHandlers) createTable(
	ctx context.Context, _ *mcp.CallToolRequest, params CreateTableParams,
) (*mcp.CallToolResult, any, error) {
	stmt, err := parseSingleStatement(params.Statement)
	if err != nil {
		return nil, nil, err
	}
	ct, ok := stmt.(*tree.CreateTable)
	if !ok {
		return nil, nil, errors.Newf("expected a CREATE TABLE statement, got %s", stmt.StatementTag())
	}

	// Block DML hidden inside CREATE TABLE AS SELECT (CTEs, bracket subqueries).
	if ct.AsSource != nil {
		if _, err := ensureSelectOnly(ct.AsSource); err != nil {
			return nil, nil, errors.Wrap(err, "CREATE TABLE AS source")
		}
	}

	if _, err := h.dm.Exec(ctx, stmt.String()); err != nil {
		return nil, nil, errors.Wrap(err, "create table")
	}
	return writeOK(map[string]any{"table": tree.AsString(&ct.Table)})
}
