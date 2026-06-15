package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestExplainQuery(t *testing.T) {
	sampleResult := &db.QueryResult{Columns: []string{"info"}, Rows: [][]any{{"plan"}}}

	t.Run("prefixes EXPLAIN", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)
		_, _, err := h.explainQuery(context.Background(), &mcp.CallToolRequest{},
			ExplainQueryParams{Query: "SELECT 1"})
		require.NoError(t, err)
		require.Truef(t, strings.HasPrefix(fq.queries[0], "EXPLAIN "),
			"expected EXPLAIN prefix: %q", fq.queries[0])
	})

	t.Run("rejects non-SELECT", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)
		_, _, err := h.explainQuery(context.Background(), &mcp.CallToolRequest{},
			ExplainQueryParams{Query: "DELETE FROM t"})
		require.Error(t, err, "expected error for non-SELECT")
	})

	t.Run("rejects multi-statement", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)
		_, _, err := h.explainQuery(context.Background(), &mcp.CallToolRequest{},
			ExplainQueryParams{Query: "SELECT 1; SELECT 2"})
		require.Error(t, err, "expected error for multi-statement")
	})

	t.Run("rejects CTE-smuggled DML", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)
		_, _, err := h.explainQuery(context.Background(), &mcp.CallToolRequest{},
			ExplainQueryParams{Query: "WITH x AS (DELETE FROM t RETURNING *) SELECT * FROM x"})
		require.Error(t, err, "expected error for CTE-smuggled DML")
		require.Empty(t, fq.queries, "no query should reach the DB")
	})

	t.Run("rejects bracket-subquery DML", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)
		_, _, err := h.explainQuery(context.Background(), &mcp.CallToolRequest{},
			ExplainQueryParams{Query: "SELECT * FROM [DELETE FROM t RETURNING *]"})
		require.Error(t, err, "expected error for bracket-subquery DML")
		require.Empty(t, fq.queries)
	})

	t.Run("rejects query exceeding maxQueryLength", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)
		oversized := "SELECT '" + strings.Repeat("x", maxQueryLength) + "'"
		_, _, err := h.explainQuery(context.Background(), &mcp.CallToolRequest{},
			ExplainQueryParams{Query: oversized})
		require.Error(t, err, "expected error for oversized query")
		require.Empty(t, fq.queries)
	})
}
