package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestInsertRows(t *testing.T) {
	t.Run("happy path passes the INSERT through and reports rows affected", func(t *testing.T) {
		fq := &fakeQuerier{rowsAffected: 3}
		h := newWriteHandlers(fq)
		res, _, err := h.insertRows(context.Background(), &mcp.CallToolRequest{},
			InsertRowsParams{Statement: "INSERT INTO users VALUES (1, 'a'), (2, 'b'), (3, 'c')"})
		require.NoError(t, err)
		require.Truef(t, strings.HasPrefix(fq.execs[0], "INSERT INTO"),
			"unexpected exec: %q", fq.execs[0])
		require.Contains(t, textOf(t, res), `"rows_affected":3`)
	})

	rejections := []struct {
		name, statement, wantContains string
	}{
		{"non-INSERT statement", "DELETE FROM users WHERE id = 1", "expected an INSERT"},
		{"multi-statement payload", "INSERT INTO a VALUES (1); INSERT INTO b VALUES (2)", "exactly one"},
		{"DML hidden in INSERT SELECT bracket subquery",
			"INSERT INTO log SELECT id FROM [DELETE FROM users RETURNING id]", "INSERT source"},
		{"DML hidden in INSERT SELECT CTE",
			"INSERT INTO log WITH d AS (DELETE FROM users RETURNING id) SELECT id FROM d", "INSERT source"},
		{"DML hidden in statement-level CTE",
			"WITH d AS (DELETE FROM users RETURNING id) INSERT INTO log (id) SELECT id FROM d",
			"INSERT CTE contains a non-SELECT"},
		{"INSERT with RETURNING clause",
			"INSERT INTO users VALUES (1, 'a') RETURNING id", "RETURNING is not supported"},
	}
	for _, tc := range rejections {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			fq := &fakeQuerier{}
			h := newWriteHandlers(fq)
			_, _, err := h.insertRows(context.Background(), &mcp.CallToolRequest{},
				InsertRowsParams{Statement: tc.statement})
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantContains)
			require.Empty(t, fq.execs, "no exec should reach the DB")
		})
	}
}
