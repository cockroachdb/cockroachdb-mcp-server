package tools

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestGetTableSchema(t *testing.T) {
	sampleResult := &db.QueryResult{
		Columns: []string{"create_statement"},
		Rows: [][]any{
			{"CREATE TABLE users (id INT PRIMARY KEY)"},
		},
	}

	t.Run("defaults schema to public and qualifies all three identifiers", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)

		_, _, err := h.getTableSchema(context.Background(), &mcp.CallToolRequest{},
			TableSchemaParams{Database: "appdb", Table: "users"})
		if err != nil {
			t.Fatalf("getTableSchema: %v", err)
		}
		got := fq.queries[0]
		if got != `SHOW CREATE TABLE "appdb"."public"."users"` {
			t.Fatalf("unexpected query: %q", got)
		}
	})

	t.Run("custom schema is honored", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)

		_, _, err := h.getTableSchema(context.Background(), &mcp.CallToolRequest{},
			TableSchemaParams{Database: "appdb", Schema: "billing", Table: "invoices"})
		if err != nil {
			t.Fatalf("getTableSchema: %v", err)
		}
		got := fq.queries[0]
		if got != `SHOW CREATE TABLE "appdb"."billing"."invoices"` {
			t.Fatalf("unexpected query: %q", got)
		}
	})

	t.Run("missing database is rejected", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)

		_, _, err := h.getTableSchema(context.Background(), &mcp.CallToolRequest{},
			TableSchemaParams{Table: "users"})
		if err == nil {
			t.Fatal("expected error for missing database")
		}
		if len(fq.queries) != 0 {
			t.Fatalf("no query should have been issued, got %d", len(fq.queries))
		}
	})

	t.Run("missing table is rejected", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)

		_, _, err := h.getTableSchema(context.Background(), &mcp.CallToolRequest{},
			TableSchemaParams{Database: "appdb"})
		if err == nil {
			t.Fatal("expected error for missing table")
		}
		if len(fq.queries) != 0 {
			t.Fatalf("no query should have been issued, got %d", len(fq.queries))
		}
	})

	t.Run("identifiers with embedded quotes are escaped", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)

		_, _, err := h.getTableSchema(context.Background(), &mcp.CallToolRequest{},
			TableSchemaParams{Database: `db"ev`, Schema: `sch"ma`, Table: `tb"l`})
		if err != nil {
			t.Fatalf("getTableSchema: %v", err)
		}
		got := fq.queries[0]
		want := `SHOW CREATE TABLE "db""ev"."sch""ma"."tb""l"`
		if got != want {
			t.Fatalf("got %q\nwant %q", got, want)
		}
	})

}
