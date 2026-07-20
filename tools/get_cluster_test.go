package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestGetCluster(t *testing.T) {
	binaryResult := &db.QueryResult{
		Columns: []string{"binary_version"},
		Rows:    [][]any{{"CockroachDB CCL v25.4.0"}},
	}
	identityResult := &db.QueryResult{
		Columns: []string{"cluster_id", "cluster_name", "active_version"},
		Rows:    [][]any{{"abc-123", "my-cluster", "25.4.0"}},
	}
	insufficientPrivilegeErr := errors.Wrap(
		&pgconn.PgError{Code: "42501", Message: "Access to crdb_internal and system is restricted."},
		"exec query")

	decode := func(t *testing.T, res *mcp.CallToolResult) map[string]any {
		t.Helper()
		var payload map[string]any
		require.NoError(t, json.Unmarshal([]byte(textOf(t, res)), &payload))
		return payload
	}

	t.Run("issues anchor and identity queries", func(t *testing.T) {
		sq := &scriptedQuerier{steps: []scriptedStep{
			{result: binaryResult},
			{result: identityResult},
		}}
		h := newHandlers(sq)
		_, _, err := h.getCluster(context.Background(), &mcp.CallToolRequest{}, struct{}{})
		require.NoError(t, err)
		require.Len(t, sq.queries, 2)
		require.Equal(t, getClusterBinaryVersionQuery, sq.queries[0])
		require.Contains(t, sq.queries[1], "crdb_internal.cluster_id()")
		require.Contains(t, sq.queries[1], "crdb_internal.cluster_name()")
		require.Contains(t, sq.queries[1], "crdb_internal.active_version()")
	})

	t.Run("returns all fields with no unavailable key on full access", func(t *testing.T) {
		sq := &scriptedQuerier{steps: []scriptedStep{
			{result: binaryResult},
			{result: identityResult},
		}}
		h := newHandlers(sq)
		res, _, err := h.getCluster(context.Background(), &mcp.CallToolRequest{}, struct{}{})
		require.NoError(t, err)
		payload := decode(t, res)
		require.Equal(t, "abc-123", payload["cluster_id"])
		require.Equal(t, "my-cluster", payload["cluster_name"])
		require.Equal(t, "CockroachDB CCL v25.4.0", payload["binary_version"])
		require.Equal(t, "25.4.0", payload["active_version"])
		require.NotContains(t, payload, "unavailable")
	})

	t.Run("gated identity builtins degrade to null with a reason", func(t *testing.T) {
		sq := &scriptedQuerier{steps: []scriptedStep{
			{result: binaryResult},
			{err: insufficientPrivilegeErr},
		}}
		h := newHandlers(sq)
		res, _, err := h.getCluster(context.Background(), &mcp.CallToolRequest{}, struct{}{})
		require.NoError(t, err)
		payload := decode(t, res)
		require.Nil(t, payload["cluster_id"])
		require.Nil(t, payload["cluster_name"])
		require.Equal(t, "CockroachDB CCL v25.4.0", payload["binary_version"])
		require.Nil(t, payload["active_version"])
		unavailable := payload["unavailable"].(map[string]any)
		require.Equal(t, restrictedInternalsReason, unavailable["cluster_id"])
		require.Equal(t, restrictedInternalsReason, unavailable["cluster_name"])
		require.Equal(t, restrictedInternalsReason, unavailable["active_version"])
	})

	t.Run("binary version query errors fail the tool", func(t *testing.T) {
		sq := &scriptedQuerier{steps: []scriptedStep{
			{err: errors.New("connection refused")},
		}}
		h := newHandlers(sq)
		_, _, err := h.getCluster(context.Background(), &mcp.CallToolRequest{}, struct{}{})
		require.ErrorContains(t, err, "get cluster")
		require.ErrorContains(t, err, "connection refused")
		require.Len(t, sq.queries, 1)
	})

	t.Run("non-42501 identity errors fail the tool", func(t *testing.T) {
		sq := &scriptedQuerier{steps: []scriptedStep{
			{result: binaryResult},
			{err: errors.New("boom")},
		}}
		h := newHandlers(sq)
		_, _, err := h.getCluster(context.Background(), &mcp.CallToolRequest{}, struct{}{})
		require.ErrorContains(t, err, "get cluster")
		require.ErrorContains(t, err, "boom")
	})

	t.Run("empty results render as null fields with no unavailable reasons", func(t *testing.T) {
		sq := &scriptedQuerier{steps: []scriptedStep{
			{result: &db.QueryResult{Columns: []string{"binary_version"}}},
			{result: &db.QueryResult{Columns: []string{"cluster_id", "cluster_name", "active_version"}}},
		}}
		h := newHandlers(sq)
		res, _, err := h.getCluster(context.Background(), &mcp.CallToolRequest{}, struct{}{})
		require.NoError(t, err)
		payload := decode(t, res)
		for _, field := range []string{"cluster_id", "cluster_name", "binary_version", "active_version"} {
			require.Nil(t, payload[field])
		}
		require.NotContains(t, payload, "unavailable")
	})
}
