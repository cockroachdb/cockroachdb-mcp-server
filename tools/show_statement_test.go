package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestShowStatement(t *testing.T) {
	sampleResult := &db.QueryResult{Columns: []string{"name"}, Rows: [][]any{{"public"}}}

	t.Run("wraps SHOW in SELECT FROM and appends default LIMIT", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)
		_, _, err := h.showStatement(context.Background(), &mcp.CallToolRequest{},
			ShowStatementParams{Query: "SHOW SCHEMAS"})
		require.NoError(t, err)
		got := fq.queries[0]
		require.Contains(t, got, "[SHOW SCHEMAS]", "query should wrap SHOW in brackets")
		require.Truef(t, strings.HasSuffix(got, "LIMIT 100"),
			"expected default LIMIT 100: %q", got)
	})

	t.Run("operational SHOW is accepted (CRDB enforces privileges)", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)
		_, _, err := h.showStatement(context.Background(), &mcp.CallToolRequest{},
			ShowStatementParams{Query: "SHOW JOBS"})
		require.NoError(t, err)
		require.Contains(t, fq.queries[0], "[SHOW JOBS]")
	})

	t.Run("rejects non-SHOW", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)
		_, _, err := h.showStatement(context.Background(), &mcp.CallToolRequest{},
			ShowStatementParams{Query: "SELECT 1"})
		require.Error(t, err, "expected error for non-SHOW")
	})

	t.Run("rejects multi-statement", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)
		_, _, err := h.showStatement(context.Background(), &mcp.CallToolRequest{},
			ShowStatementParams{Query: "SHOW SCHEMAS; SHOW DATABASES"})
		require.Error(t, err, "expected error for multi-statement")
	})

	t.Run("respects caller-supplied limit and offset", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)
		limit, offset := int64(7), int64(3)
		_, _, err := h.showStatement(context.Background(), &mcp.CallToolRequest{},
			ShowStatementParams{
				Query:            "SHOW SCHEMAS",
				PaginationParams: PaginationParams{Limit: &limit, Offset: &offset},
			})
		require.NoError(t, err)
		require.Truef(t, strings.HasSuffix(fq.queries[0], "LIMIT 7 OFFSET 3"),
			"expected LIMIT 7 OFFSET 3: %q", fq.queries[0])
	})

	t.Run("rejects query exceeding maxQueryLength", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)
		oversized := "SHOW '" + strings.Repeat("x", maxQueryLength) + "'"
		_, _, err := h.showStatement(context.Background(), &mcp.CallToolRequest{},
			ShowStatementParams{Query: oversized})
		require.Error(t, err, "expected error for oversized query")
		require.Empty(t, fq.queries)
	})
}
