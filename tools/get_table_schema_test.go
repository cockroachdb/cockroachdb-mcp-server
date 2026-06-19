package tools

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
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
		require.NoError(t, err)
		require.Equal(t, `SHOW CREATE TABLE "appdb"."public"."users"`, fq.queries[0])
	})

	t.Run("custom schema is honored", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)

		_, _, err := h.getTableSchema(context.Background(), &mcp.CallToolRequest{},
			TableSchemaParams{Database: "appdb", Schema: "billing", Table: "invoices"})
		require.NoError(t, err)
		require.Equal(t, `SHOW CREATE TABLE "appdb"."billing"."invoices"`, fq.queries[0])
	})

	t.Run("missing database is rejected", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)

		_, _, err := h.getTableSchema(context.Background(), &mcp.CallToolRequest{},
			TableSchemaParams{Table: "users"})
		require.Error(t, err, "expected error for missing database")
		require.Empty(t, fq.queries, "no query should have been issued")
	})

	t.Run("missing table is rejected", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)

		_, _, err := h.getTableSchema(context.Background(), &mcp.CallToolRequest{},
			TableSchemaParams{Database: "appdb"})
		require.Error(t, err, "expected error for missing table")
		require.Empty(t, fq.queries, "no query should have been issued")
	})

	t.Run("identifiers with embedded quotes are escaped", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)

		_, _, err := h.getTableSchema(context.Background(), &mcp.CallToolRequest{},
			TableSchemaParams{Database: `db"ev`, Schema: `sch"ma`, Table: `tb"l`})
		require.NoError(t, err)
		require.Equal(t, `SHOW CREATE TABLE "db""ev"."sch""ma"."tb""l"`, fq.queries[0])
	})
}
