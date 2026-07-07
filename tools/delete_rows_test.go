package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestDeleteRows(t *testing.T) {
	t.Run("a valid DELETE passes through and reports rows affected", func(t *testing.T) {
		fq := &fakeQuerier{rowsAffected: 4}
		h := newWriteHandlers(fq)
		res, _, err := h.deleteRows(context.Background(), &mcp.CallToolRequest{},
			DeleteRowsParams{Statement: "DELETE FROM users WHERE status = 'stale'"})
		require.NoError(t, err)
		require.Truef(t, strings.HasPrefix(fq.execs[0], "DELETE FROM"),
			"unexpected exec: %q", fq.execs[0])
		require.Contains(t, textOf(t, res), `"rows_affected":4`)
	})

	t.Run("RETURNING routes through Query and returns the rows", func(t *testing.T) {
		fq := &fakeQuerier{result: &db.QueryResult{
			Columns: []string{"id"},
			Rows:    [][]any{{int64(1)}, {int64(2)}},
		}}
		h := newWriteHandlers(fq)
		res, _, err := h.deleteRows(context.Background(), &mcp.CallToolRequest{},
			DeleteRowsParams{Statement: "DELETE FROM users WHERE status = 'stale' RETURNING id"})
		require.NoError(t, err)
		require.Len(t, fq.queries, 1, "RETURNING should use Query, not Exec")
		require.Empty(t, fq.execs)
		text := textOf(t, res)
		require.Contains(t, text, `"rows_affected":2`)
		require.Contains(t, text, `"id":1`)
	})

	t.Run("allows plain SELECT subqueries in WHERE", func(t *testing.T) {
		fq := &fakeQuerier{rowsAffected: 1}
		h := newWriteHandlers(fq)
		_, _, err := h.deleteRows(context.Background(), &mcp.CallToolRequest{},
			DeleteRowsParams{Statement: "DELETE FROM users WHERE id IN (SELECT id FROM inactive)"})
		require.NoError(t, err)
		require.Len(t, fq.execs, 1)
	})

	rejections := []rejectionCase{
		{"non-DELETE statement", "UPDATE users SET name = 'b' WHERE id = 1", "expected a DELETE"},
		{"DELETE without WHERE", "DELETE FROM users", "WHERE clause"},
		{"multi-statement payload",
			"DELETE FROM a WHERE id = 1; DELETE FROM b WHERE id = 2", "exactly one"},
		{"DML hidden in statement-level CTE",
			"WITH d AS (DELETE FROM audit RETURNING id) DELETE FROM users WHERE id IN (SELECT id FROM d)",
			"CTE contains a non-SELECT"},
		{"DML hidden in WHERE bracket subquery",
			"DELETE FROM users WHERE id IN (SELECT id FROM [DELETE FROM audit RETURNING id])",
			"only SELECT statements are allowed"},
		{"DML hidden in USING clause",
			"DELETE FROM users USING [UPDATE audit SET seen = true RETURNING id] AS u WHERE users.id = u.id",
			"only SELECT statements are allowed"},
	}
	for _, tc := range rejections {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			fq := &fakeQuerier{}
			_, _, err := newWriteHandlers(fq).deleteRows(context.Background(), &mcp.CallToolRequest{},
				DeleteRowsParams{Statement: tc.statement})
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantContains)
			require.Empty(t, fq.execs, "no exec should reach the DB")
		})
	}
}
