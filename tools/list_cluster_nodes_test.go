package tools

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestListClusterNodes(t *testing.T) {
	t.Run("query references gossip and node status tables", func(t *testing.T) {
		fq := &fakeQuerier{result: &db.QueryResult{Columns: []string{"node_id"}}}
		h := newHandlers(fq)
		_, _, err := h.listClusterNodes(context.Background(), &mcp.CallToolRequest{}, struct{}{})
		require.NoError(t, err)
		got := fq.queries[0]
		for _, want := range []string{
			"crdb_internal.gossip_nodes",
			"crdb_internal.kv_node_status",
			"address",
			"is_live",
			"locality",
		} {
			require.Contains(t, got, want)
		}
	})

	t.Run("42501 errors explain both possible causes", func(t *testing.T) {
		fq := &fakeQuerier{err: errors.Wrap(
			&pgconn.PgError{Code: "42501", Message: "Access to crdb_internal and system is restricted."},
			"exec query")}
		h := newHandlers(fq)
		_, _, err := h.listClusterNodes(context.Background(), &mcp.CallToolRequest{}, struct{}{})
		require.ErrorContains(t, err, "VIEWCLUSTERMETADATA")
		require.ErrorContains(t, err, "allow_unsafe_internals")
	})

	t.Run("other errors are wrapped unchanged", func(t *testing.T) {
		fq := &fakeQuerier{err: errors.New("connection refused")}
		h := newHandlers(fq)
		_, _, err := h.listClusterNodes(context.Background(), &mcp.CallToolRequest{}, struct{}{})
		require.ErrorContains(t, err, "list cluster nodes")
		require.ErrorContains(t, err, "connection refused")
		require.NotContains(t, err.Error(), "allow_unsafe_internals")
	})
}
