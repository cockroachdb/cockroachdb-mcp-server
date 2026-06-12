package tools

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestListClusterNodes(t *testing.T) {
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
}
