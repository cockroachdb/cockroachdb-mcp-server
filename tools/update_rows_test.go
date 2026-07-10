package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestUpdateRows(t *testing.T) {
	t.Run("a valid UPDATE passes through and reports rows affected", func(t *testing.T) {
		fq := &fakeQuerier{rowsAffected: 2}
		h := newWriteHandlers(fq)
		res, _, err := h.updateRows(context.Background(), &mcp.CallToolRequest{},
			UpdateRowsParams{Statement: "UPDATE users SET name = 'b' WHERE id = 1"})
		require.NoError(t, err)
		require.Truef(t, strings.HasPrefix(fq.execs[0], "UPDATE"),
			"unexpected exec: %q", fq.execs[0])
		require.Contains(t, textOf(t, res), `"rows_affected":2`)
	})

	t.Run("RETURNING routes through Query and returns the rows", func(t *testing.T) {
		fq := &fakeQuerier{result: &db.QueryResult{
			Columns: []string{"id", "name"},
			Rows:    [][]any{{int64(1), "b"}},
		}}
		h := newWriteHandlers(fq)
		res, _, err := h.updateRows(context.Background(), &mcp.CallToolRequest{},
			UpdateRowsParams{Statement: "UPDATE users SET name = 'b' WHERE id = 1 RETURNING id, name"})
		require.NoError(t, err)
		require.Len(t, fq.queries, 1, "RETURNING should use Query, not Exec")
		require.Empty(t, fq.execs)
		text := textOf(t, res)
		require.Contains(t, text, `"rows_affected":1`)
		require.Contains(t, text, `"name":"b"`)
	})

	t.Run("allows plain SELECT subqueries in SET and WHERE", func(t *testing.T) {
		fq := &fakeQuerier{rowsAffected: 1}
		h := newWriteHandlers(fq)
		_, _, err := h.updateRows(context.Background(), &mcp.CallToolRequest{},
			UpdateRowsParams{Statement: "UPDATE users SET name = (SELECT name FROM legacy WHERE legacy.id = users.id) WHERE id IN (SELECT id FROM active)"})
		require.NoError(t, err)
		require.Len(t, fq.execs, 1)
	})

	rejections := []rejectionCase{
		{"non-UPDATE statement", "DELETE FROM users WHERE id = 1", "expected an UPDATE"},
		{"UPDATE without WHERE", "UPDATE users SET name = 'b'", "WHERE clause"},
		{"multi-statement payload",
			"UPDATE a SET x = 1 WHERE id = 1; UPDATE b SET x = 2 WHERE id = 2", "exactly one"},
		{"DML hidden in statement-level CTE",
			"WITH d AS (DELETE FROM audit RETURNING id) UPDATE users SET name = 'b' WHERE id IN (SELECT id FROM d)",
			"CTE contains a non-SELECT"},
		{"DML hidden in WHERE bracket subquery",
			"UPDATE users SET name = 'b' WHERE id IN (SELECT id FROM [DELETE FROM audit RETURNING id])",
			"only SELECT statements are allowed"},
		{"DML hidden in FROM clause",
			"UPDATE users SET name = 'b' FROM [DELETE FROM audit RETURNING id] AS d WHERE users.id = d.id",
			"only SELECT statements are allowed"},
	}
	for _, tc := range rejections {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			fq := &fakeQuerier{}
			_, _, err := newWriteHandlers(fq).updateRows(context.Background(), &mcp.CallToolRequest{},
				UpdateRowsParams{Statement: tc.statement})
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantContains)
			require.Empty(t, fq.execs, "no exec should reach the DB")
		})
	}
}
