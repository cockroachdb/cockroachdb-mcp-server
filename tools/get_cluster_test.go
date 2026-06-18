package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestGetCluster(t *testing.T) {
	sampleResult := &db.QueryResult{
		Columns: []string{"cluster_id", "cluster_name", "binary_version", "active_version"},
		Rows: [][]any{
			{"abc-123", "my-cluster", "v24.1.0", "24.1"},
		},
	}

	t.Run("query references cluster identity builtins", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)
		_, _, err := h.getCluster(context.Background(), &mcp.CallToolRequest{}, struct{}{})
		require.NoError(t, err)
		got := fq.queries[0]
		for _, want := range []string{
			"crdb_internal.cluster_id()",
			"crdb_internal.cluster_name()",
			"version()",
			"crdb_internal.active_version()",
		} {
			require.Contains(t, got, want)
		}
	})

	t.Run("single row is returned in the rows array", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)
		res, _, err := h.getCluster(context.Background(), &mcp.CallToolRequest{}, struct{}{})
		require.NoError(t, err)
		var payload struct {
			Rows []map[string]any `json:"rows"`
		}
		require.NoError(t, json.Unmarshal([]byte(textOf(t, res)), &payload))
		require.Len(t, payload.Rows, 1)
		require.Equal(t, "my-cluster", payload.Rows[0]["cluster_name"])
	})
}
