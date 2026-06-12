package tools

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestShowRunningQueries(t *testing.T) {
	fq := &fakeQuerier{result: &db.QueryResult{Columns: []string{"query_id"}}}
	h := newHandlers(fq)
	_, _, err := h.showRunningQueries(context.Background(), &mcp.CallToolRequest{}, ShowRunningQueriesParams{})
	require.NoError(t, err)
	got := fq.queries[0]
	for _, want := range []string{
		"[SHOW CLUSTER STATEMENTS]",
		"user_name",
		"ORDER BY start DESC",
	} {
		require.Contains(t, got, want)
	}
}
