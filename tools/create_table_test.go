package tools

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestCreateTable(t *testing.T) {
	t.Run("happy path passes the CREATE TABLE through", func(t *testing.T) {
		fq := &fakeQuerier{}
		h := newWriteHandlers(fq)
		_, _, err := h.createTable(context.Background(), &mcp.CallToolRequest{},
			CreateTableParams{Statement: "CREATE TABLE users (id INT PRIMARY KEY, name STRING)"})
		require.NoError(t, err)
		require.Contains(t, fq.execs[0], "CREATE TABLE")
		require.Contains(t, fq.execs[0], "users")
	})

	rejections := []struct {
		name, statement, wantContains string
	}{
		{"non-CREATE-TABLE statement", "SELECT 1", "expected a CREATE TABLE"},
		{"multi-statement payload", "CREATE TABLE a (id INT); CREATE TABLE b (id INT)", "exactly one"},
		{"DML hidden in AS SELECT bracket subquery",
			"CREATE TABLE x AS SELECT * FROM [DELETE FROM y RETURNING *]", "CREATE TABLE AS source"},
		{"DML hidden in AS SELECT CTE",
			"CREATE TABLE x AS WITH d AS (DELETE FROM y RETURNING *) SELECT * FROM d", "CREATE TABLE AS source"},
	}
	for _, tc := range rejections {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			fq := &fakeQuerier{}
			h := newWriteHandlers(fq)
			_, _, err := h.createTable(context.Background(), &mcp.CallToolRequest{},
				CreateTableParams{Statement: tc.statement})
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantContains)
			require.Empty(t, fq.execs, "no exec should reach the DB")
		})
	}
}
