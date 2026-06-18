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
		require.Equal(t, "SELECT 1 LIMIT 7", fq.queries[0])
	})

	rejections := []struct {
		name, query string
	}{
		{"non-SELECT", "INSERT INTO t VALUES (1)"},
		{"multi-statement", "SELECT 1; SELECT 2"},
		{"empty query", ""},
		{"oversized query", "SELECT '" + strings.Repeat("x", maxQueryLength) + "'"},
		{"LIMIT above MaxRowsCount", "SELECT 1 LIMIT 999999"},
		{"LIMIT ALL", "SELECT 1 LIMIT ALL"},
	}
	for _, tc := range rejections {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			fq := &fakeQuerier{result: sampleResult}
			h := newHandlers(fq)
			_, _, err := h.selectQuery(context.Background(), &mcp.CallToolRequest{},
				SelectQueryParams{Query: tc.query})
			require.Error(t, err)
			require.Empty(t, fq.queries, "no query should reach the DB")
		})
	}
}
