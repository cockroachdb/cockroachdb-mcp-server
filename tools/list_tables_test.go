package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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
		if err != nil {
			t.Fatalf("listTables: %v", err)
		}
		got := fq.queries[0]
		if !strings.Contains(got, `[SHOW TABLES FROM "appdb"]`) {
			t.Fatalf("query should quote the database identifier: %q", got)
		}
		if !strings.HasSuffix(got, "LIMIT 100") {
			t.Fatalf("query should append default LIMIT 100: %q", got)
		}
	})

	t.Run("missing database is rejected before any query", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)

		_, _, err := h.listTables(context.Background(), &mcp.CallToolRequest{}, ListTablesParams{})
		if err == nil {
			t.Fatal("expected error for empty database")
		}
		if len(fq.queries) != 0 {
			t.Fatalf("no query should have been issued, got %d", len(fq.queries))
		}
	})

	t.Run("database identifier with embedded quote is escaped", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)

		_, _, err := h.listTables(context.Background(), &mcp.CallToolRequest{}, ListTablesParams{Database: `na"me`})
		if err != nil {
			t.Fatalf("listTables: %v", err)
		}
		got := fq.queries[0]
		if !strings.Contains(got, `[SHOW TABLES FROM "na""me"]`) {
			t.Fatalf("embedded quote should be escaped: %q", got)
		}
	})

}
