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

func TestExplainQuery(t *testing.T) {
	sampleResult := &db.QueryResult{Columns: []string{"info"}, Rows: [][]any{{"plan"}}}

	wrapped := []struct {
		name, query string
	}{
		{"SELECT", "SELECT 1"},
		{"INSERT", "INSERT INTO t VALUES (1)"},
		{"UPDATE", "UPDATE t SET a = 1"},
		{"DELETE", "DELETE FROM t"},
		{"CREATE TABLE", "CREATE TABLE t (a INT)"},
	}
	for _, tc := range wrapped {
		t.Run("prefixes EXPLAIN for "+tc.name, func(t *testing.T) {
			fq := &fakeQuerier{result: sampleResult}
			h := newHandlers(fq)
			_, _, err := h.explainQuery(context.Background(), &mcp.CallToolRequest{},
				ExplainQueryParams{Query: tc.query})
			require.NoError(t, err)
			require.Truef(t, strings.HasPrefix(fq.queries[0], "EXPLAIN "),
				"expected EXPLAIN prefix, got %q", fq.queries[0])
		})
	}

	passthrough := []struct {
		name, query string
	}{
		{"plain EXPLAIN", "EXPLAIN SELECT 1"},
		{"EXPLAIN VERBOSE", "EXPLAIN (VERBOSE) SELECT 1"},
		{"EXPLAIN DISTSQL", "EXPLAIN (DISTSQL) SELECT 1"},
		{"EXPLAIN TYPES", "EXPLAIN (TYPES) SELECT 1"},
	}
	for _, tc := range passthrough {
		t.Run("passes through "+tc.name, func(t *testing.T) {
			fq := &fakeQuerier{result: sampleResult}
			h := newHandlers(fq)
			_, _, err := h.explainQuery(context.Background(), &mcp.CallToolRequest{},
				ExplainQueryParams{Query: tc.query})
			require.NoError(t, err)
			require.Falsef(t, strings.HasPrefix(fq.queries[0], "EXPLAIN EXPLAIN"),
				"already-EXPLAIN input should not be double-wrapped, got %q", fq.queries[0])
		})
	}

	t.Run("DB errors are wrapped with the tool name", func(t *testing.T) {
		fq := &fakeQuerier{err: errors.New("boom")}
		h := newHandlers(fq)
		_, _, err := h.explainQuery(context.Background(), &mcp.CallToolRequest{},
			ExplainQueryParams{Query: "SELECT 1"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "explain query")
		require.Contains(t, err.Error(), "boom")
	})

	rejections := []struct {
		name, query string
	}{
		{"multi-statement", "SELECT 1; SELECT 2"},
		{"empty query", ""},
		{"oversized query", "SELECT '" + strings.Repeat("x", maxQueryLength) + "'"},
		{"EXPLAIN ANALYZE", "EXPLAIN ANALYZE DELETE FROM t"},
		{"EXPLAIN ANALYZE DEBUG", "EXPLAIN ANALYZE (DEBUG) SELECT 1"},
	}
	for _, tc := range rejections {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			fq := &fakeQuerier{result: sampleResult}
			h := newHandlers(fq)
			_, _, err := h.explainQuery(context.Background(), &mcp.CallToolRequest{},
				ExplainQueryParams{Query: tc.query})
			require.Error(t, err)
			require.Empty(t, fq.queries, "no query should reach the DB")
		})
	}
}
