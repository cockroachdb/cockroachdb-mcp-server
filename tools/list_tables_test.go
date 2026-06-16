package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestListTables(t *testing.T) {
	sampleResult := &db.QueryResult{
		Columns: []string{"schema_name", "table_name"},
		Rows: [][]any{
			{"public", "users"},
			{"public", "orders"},
		},
	}

	t.Run("happy path quotes the database identifier and applies default LIMIT", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)

		_, _, err := h.listTables(context.Background(), &mcp.CallToolRequest{}, ListTablesParams{Database: "appdb"})
		require.NoError(t, err)
		got := fq.queries[0]
		require.Contains(t, got, `[SHOW TABLES FROM "appdb"]`)
		require.Truef(t, strings.HasSuffix(got, "LIMIT 100"),
			"query should append default LIMIT 100: %q", got)
	})

	t.Run("missing database is rejected before any query", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)

		_, _, err := h.listTables(context.Background(), &mcp.CallToolRequest{}, ListTablesParams{})
		require.Error(t, err, "expected error for empty database")
		require.Empty(t, fq.queries, "no query should have been issued")
	})

	t.Run("database identifier with embedded quote is escaped", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)

		_, _, err := h.listTables(context.Background(), &mcp.CallToolRequest{}, ListTablesParams{Database: `na"me`})
		require.NoError(t, err)
		require.Contains(t, fq.queries[0], `[SHOW TABLES FROM "na""me"]`)
	})
}
