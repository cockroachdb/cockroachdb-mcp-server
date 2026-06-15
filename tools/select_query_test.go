package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestSelectQuery(t *testing.T) {
	sampleResult := &db.QueryResult{Columns: []string{"x"}, Rows: [][]any{{int64(1)}}}

	t.Run("happy path appends default LIMIT", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)
		_, _, err := h.selectQuery(context.Background(), &mcp.CallToolRequest{},
			SelectQueryParams{Query: "SELECT 1"})
		require.NoError(t, err)
		require.Truef(t, strings.HasSuffix(fq.queries[0], "LIMIT 100"),
			"expected default LIMIT 100, got %q", fq.queries[0])
	})

	t.Run("respects caller-supplied LIMIT", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)
		_, _, err := h.selectQuery(context.Background(), &mcp.CallToolRequest{},
			SelectQueryParams{Query: "SELECT 1 LIMIT 7"})
		require.NoError(t, err)
		require.NotContains(t, fq.queries[0], "LIMIT 100", "should not append default LIMIT when one is supplied")
		require.Contains(t, fq.queries[0], "LIMIT 7", "should preserve caller LIMIT")
	})

	t.Run("rejects non-SELECT", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)
		_, _, err := h.selectQuery(context.Background(), &mcp.CallToolRequest{},
			SelectQueryParams{Query: "INSERT INTO t VALUES (1)"})
		require.Error(t, err, "expected error for non-SELECT")
		require.Empty(t, fq.queries, "no query should reach the DB")
	})

	t.Run("rejects multi-statement", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)
		_, _, err := h.selectQuery(context.Background(), &mcp.CallToolRequest{},
			SelectQueryParams{Query: "SELECT 1; SELECT 2"})
		require.Error(t, err, "expected error for multi-statement")
	})

	t.Run("rejects empty query", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)
		_, _, err := h.selectQuery(context.Background(), &mcp.CallToolRequest{},
			SelectQueryParams{Query: ""})
		require.Error(t, err, "expected error for empty query")
	})

	t.Run("rejects query exceeding maxQueryLength", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)
		oversized := "SELECT '" + strings.Repeat("x", maxQueryLength) + "'"
		_, _, err := h.selectQuery(context.Background(), &mcp.CallToolRequest{},
			SelectQueryParams{Query: oversized})
		require.Error(t, err, "expected error for oversized query")
		require.Empty(t, fq.queries)
	})

	t.Run("rejects caller LIMIT above MaxRowsCount", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)
		_, _, err := h.selectQuery(context.Background(), &mcp.CallToolRequest{},
			SelectQueryParams{Query: "SELECT 1 LIMIT 999999"})
		require.Error(t, err, "expected error for LIMIT > MaxRowsCount")
		require.Empty(t, fq.queries)
	})

	t.Run("rejects LIMIT ALL", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)
		_, _, err := h.selectQuery(context.Background(), &mcp.CallToolRequest{},
			SelectQueryParams{Query: "SELECT 1 LIMIT ALL"})
		require.Error(t, err, "expected error for LIMIT ALL")
		require.Empty(t, fq.queries)
	})
}
