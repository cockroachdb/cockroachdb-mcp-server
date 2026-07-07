package tools

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	crdbparser "github.com/cockroachdb/cockroachdb-parser/pkg/sql/parser"
	"github.com/cockroachdb/cockroachdb-parser/pkg/sql/sem/tree"
	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// defaultRowLimit is the LIMIT applied to list-style tools when the caller
// does not supply one. The hard ceiling is config.MaxRowsCount.
const defaultRowLimit int64 = 100

// maxQueryLength bounds the size of agent-supplied SQL accepted by the
// query execution tools.
const maxQueryLength int = 100000

// parseSingleStatement parses sql via cockroachdb-parser and returns the AST.
// Rejects empty input, payloads larger than maxQueryLength, syntactically
// invalid SQL, and payloads that parse to more than one statement.
func parseSingleStatement(sql string) (tree.Statement, error) {
	if len(sql) == 0 {
		return nil, errors.New("query is required")
	}
	if len(sql) > maxQueryLength {
		return nil, errors.Newf("query exceeds maximum length of %d characters", maxQueryLength)
	}
	stmts, err := crdbparser.Parse(sql)
	if err != nil {
		return nil, errors.Wrap(err, "invalid SQL syntax")
	}
	if len(stmts) != 1 {
		return nil, errors.Newf("query must contain exactly one statement, got %d", len(stmts))
	}
	return stmts[0].AST, nil
}

// ensureSelectOnly walks the AST and rejects DML hidden inside CTEs, bracket
// subqueries, or expression subqueries (e.g. `WITH x AS (DELETE ... RETURNING
// *) SELECT * FROM x`). AST-level only; side-effecting builtins inside a
// SELECT are gated by the connecting role's privileges.
func ensureSelectOnly(stmt tree.Statement) (*tree.Select, error) {
	sel, ok := stmt.(*tree.Select)
	if !ok {
		return nil, errors.Newf("only SELECT statements are allowed, got %s", stmt.StatementTag())
	}
	if sel.With != nil {
		for _, cte := range sel.With.CTEList {
			if _, err := ensureSelectOnly(cte.Stmt); err != nil {
				return nil, errors.Wrap(err, "CTE contains a non-SELECT statement")
			}
		}
	}
	switch sc := sel.Select.(type) {
	case *tree.SelectClause:
		for _, te := range sc.From.Tables {
			if err := walkTableExprReadOnly(te); err != nil {
				return nil, err
			}
		}
	case *tree.UnionClause:
		if _, err := ensureSelectOnly(sc.Left); err != nil {
			return nil, err
		}
		if _, err := ensureSelectOnly(sc.Right); err != nil {
			return nil, err
		}
	case *tree.ParenSelect:
		if _, err := ensureSelectOnly(sc.Select); err != nil {
			return nil, err
		}
	}
	// Covers Subqueries in expression position (projection list, WHERE);
	// walkTableExprReadOnly covers FROM-clause Subqueries.
	_, err := tree.SimpleStmtVisit(sel, func(e tree.Expr) (bool, tree.Expr, error) {
		sub, ok := e.(*tree.Subquery)
		if !ok {
			return true, e, nil
		}
		if ps, ok := sub.Select.(*tree.ParenSelect); ok {
			if _, err := ensureSelectOnly(ps.Select); err != nil {
				return false, e, err
			}
		}
		return true, e, nil
	})
	if err != nil {
		return nil, err
	}
	return sel, nil
}

// walkTableExprReadOnly rejects non-SELECT statements hidden inside
// FROM-clause bracket or parenthesised subqueries.
func walkTableExprReadOnly(expr tree.TableExpr) error {
	switch t := expr.(type) {
	case *tree.AliasedTableExpr:
		return walkTableExprReadOnly(t.Expr)
	case *tree.JoinTableExpr:
		if err := walkTableExprReadOnly(t.Left); err != nil {
			return err
		}
		return walkTableExprReadOnly(t.Right)
	case *tree.ParenTableExpr:
		return walkTableExprReadOnly(t.Expr)
	case *tree.StatementSource:
		_, err := ensureSelectOnly(t.Statement)
		return err
	case *tree.Subquery:
		if ps, ok := t.Select.(*tree.ParenSelect); ok {
			_, err := ensureSelectOnly(ps.Select)
			return err
		}
	}
	return nil
}

// ensureNoHiddenDML rejects DML hidden inside a mutation statement's CTEs
// (WITH d AS (DELETE ...) UPDATE ...), joined table expressions (UPDATE ...
// FROM, DELETE ... USING), or expression subqueries (SET/WHERE).
func ensureNoHiddenDML(stmt tree.Statement) error {
	var with *tree.With
	var tables tree.TableExprs
	switch s := stmt.(type) {
	case *tree.Update:
		with, tables = s.With, s.From
	case *tree.Delete:
		with, tables = s.With, s.Using
	}
	if with != nil {
		for _, cte := range with.CTEList {
			if _, err := ensureSelectOnly(cte.Stmt); err != nil {
				return errors.Wrap(err, "CTE contains a non-SELECT statement")
			}
		}
	}
	for _, te := range tables {
		if err := walkTableExprReadOnly(te); err != nil {
			return err
		}
	}
	_, err := tree.SimpleStmtVisit(stmt, func(e tree.Expr) (bool, tree.Expr, error) {
		sub, ok := e.(*tree.Subquery)
		if !ok {
			return true, e, nil
		}
		if ps, ok := sub.Select.(*tree.ParenSelect); ok {
			if _, err := ensureSelectOnly(ps.Select); err != nil {
				return false, e, err
			}
		}
		return true, e, nil
	})
	return err
}

// validateLimitClause rejects a SELECT LIMIT clause that is negative, uses
// LIMIT ALL, is not a numeric literal, or exceeds maxLimit.
func validateLimitClause(limit *tree.Limit, maxLimit int64) error {
	if limit == nil {
		return nil
	}
	if limit.LimitAll {
		return errors.Newf("LIMIT ALL is not allowed; maximum LIMIT is %d", maxLimit)
	}
	if limit.Count == nil {
		return nil
	}
	numVal, ok := limit.Count.(*tree.NumVal)
	if !ok {
		return errors.Newf("LIMIT must be a numeric literal; maximum LIMIT is %d", maxLimit)
	}
	n, err := numVal.AsInt64()
	if err != nil {
		return errors.Wrapf(err, "invalid LIMIT value; maximum LIMIT is %d", maxLimit)
	}
	if n < 0 {
		return errors.New("LIMIT must be zero or positive")
	}
	if n > maxLimit {
		return errors.Newf("LIMIT %d exceeds maximum of %d", n, maxLimit)
	}
	return nil
}

// validateShowStatement enforces that stmt is a SHOW statement.
func validateShowStatement(stmt tree.Statement) error {
	tag := stmt.StatementTag()
	if tag != "SHOW" && !strings.HasPrefix(tag, "SHOW ") {
		return errors.Newf("only SHOW statements are allowed, got %s", tag)
	}
	return nil
}

// MCPQueryResult is the response shape returned by read tools.
type MCPQueryResult struct {
	Rows []json.RawMessage `json:"rows"`
}

// runMutation executes a validated INSERT/UPDATE/DELETE and renders its result.
// A RETURNING statement runs through Query so its rows reach the caller; Exec,
// used otherwise, reports only the affected-row count and drops any rows.
func (h *ToolHandlers) runMutation(
	ctx context.Context, stmt tree.Statement, errCtx string,
) (*mcp.CallToolResult, any, error) {
	var table tree.TableExpr
	var returning tree.ReturningClause
	switch s := stmt.(type) {
	case *tree.Insert:
		table, returning = s.Table, s.Returning
	case *tree.Update:
		table, returning = s.Table, s.Returning
	case *tree.Delete:
		table, returning = s.Table, s.Returning
	}

	tableName := tree.AsString(table)
	if tree.HasReturningClause(returning) {
		res, err := h.dm.Query(ctx, stmt.String())
		if err != nil {
			return nil, nil, errors.Wrap(err, errCtx)
		}
		return mutationOK(tableName, int64(len(res.Rows)), res)
	}
	rows, err := h.dm.Exec(ctx, stmt.String())
	if err != nil {
		return nil, nil, errors.Wrap(err, errCtx)
	}
	return mutationOK(tableName, rows, nil)
}

// mutationOK renders a DML tool's result as an MCP response; qr carries the
// RETURNING rows when the statement ran through Query, nil otherwise.
func mutationOK(table string, rowsAffected int64, qr *db.QueryResult) (*mcp.CallToolResult, any, error) {
	payload := map[string]any{
		"table":         table,
		"rows_affected": rowsAffected,
	}
	if qr != nil {
		rows := make([]map[string]any, 0, len(qr.Rows))
		for _, r := range qr.Rows {
			m := make(map[string]any, len(qr.Columns))
			for i, col := range qr.Columns {
				m[col] = r[i]
			}
			rows = append(rows, m)
		}
		payload["rows"] = rows
	}
	return writeOK(payload)
}

// writeOK renders a write-tool success payload as an MCP tool response.
func writeOK(payload map[string]any) (*mcp.CallToolResult, any, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, errors.Wrap(err, "marshal write result")
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, nil, nil
}

// queryResultToMCP renders a db.QueryResult as an MCP tool response.
func queryResultToMCP(qr *db.QueryResult) (*mcp.CallToolResult, error) {
	rows := make([]json.RawMessage, 0, len(qr.Rows))
	for _, row := range qr.Rows {
		m := make(map[string]any, len(qr.Columns))
		for i, col := range qr.Columns {
			m[col] = row[i]
		}
		raw, err := json.Marshal(m)
		if err != nil {
			return nil, errors.Wrap(err, "marshal row")
		}
		rows = append(rows, raw)
	}
	data, err := json.Marshal(&MCPQueryResult{Rows: rows})
	if err != nil {
		return nil, errors.Wrap(err, "marshal query result")
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, nil
}

// applyLimitOffset appends LIMIT and OFFSET clauses via db.SafeFormat.
// Non-positive limits and negative offsets are rejected, and limits are capped
// at the server-configured MaxRowsCount.
func (h *ToolHandlers) applyLimitOffset(query string, limit, offset *int64) (string, error) {
	if limit != nil && *limit <= 0 {
		return "", errors.New("LIMIT must be a positive integer")
	}
	if offset != nil && *offset < 0 {
		return "", errors.New("OFFSET must be zero or positive")
	}
	limitVal := defaultRowLimit
	if limit != nil {
		limitVal = min(*limit, h.cfg.MaxRowsCount)
	}
	var offsetVal int64
	if offset != nil && *offset > 0 {
		offsetVal = *offset
	}
	if offsetVal > 0 {
		return db.SafeFormat("%1 LIMIT %2 OFFSET %3", db.SQL(query), limitVal, offsetVal)
	}
	return db.SafeFormat("%1 LIMIT %2", db.SQL(query), limitVal)
}
