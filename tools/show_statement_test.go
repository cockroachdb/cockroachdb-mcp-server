package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/cockroachdb/errors"
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
			"expected default LIMIT 100, got %q", got)
	})

	t.Run("operational SHOW is accepted (CRDB enforces privileges)", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)
		_, _, err := h.showStatement(context.Background(), &mcp.CallToolRequest{},
			ShowStatementParams{Query: "SHOW JOBS"})
		require.NoError(t, err)
		require.Contains(t, fq.queries[0], "[SHOW JOBS]")
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
			"expected LIMIT 7 OFFSET 3, got %q", fq.queries[0])
	})

	t.Run("DB errors are wrapped with the tool name", func(t *testing.T) {
		fq := &fakeQuerier{err: errors.New("boom")}
		h := newHandlers(fq)
		_, _, err := h.showStatement(context.Background(), &mcp.CallToolRequest{},
			ShowStatementParams{Query: "SHOW SCHEMAS"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "show statement")
		require.Contains(t, err.Error(), "boom")
	})

	rejections := []struct {
		name, query string
	}{
		{"non-SHOW", "SELECT 1"},
		{"multi-statement", "SHOW SCHEMAS; SHOW DATABASES"},
		{"oversized query", "SHOW '" + strings.Repeat("x", maxQueryLength) + "'"},
	}
	for _, tc := range rejections {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			fq := &fakeQuerier{result: sampleResult}
			h := newHandlers(fq)
			_, _, err := h.showStatement(context.Background(), &mcp.CallToolRequest{},
				ShowStatementParams{Query: tc.query})
			require.Error(t, err)
			require.Empty(t, fq.queries, "no query should reach the DB")
		})
	}
}
