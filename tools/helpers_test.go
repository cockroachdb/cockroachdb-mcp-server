package tools

import (
	"encoding/json"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/stretchr/testify/require"
)

func TestApplyLimitOffset(t *testing.T) {
	mk := func(v int64) *int64 { return &v }
	h := newHandlers(nil)

	t.Run("uses default when limit is nil", func(t *testing.T) {
		got, err := h.applyLimitOffset("SELECT 1", nil, nil)
		require.NoError(t, err)
		require.Equal(t, "SELECT 1 LIMIT 100", got)
	})

	t.Run("respects explicit limit", func(t *testing.T) {
		got, err := h.applyLimitOffset("SELECT 1", mk(7), nil)
		require.NoError(t, err)
		require.Equal(t, "SELECT 1 LIMIT 7", got)
	})

	t.Run("appends offset", func(t *testing.T) {
		got, err := h.applyLimitOffset("SELECT 1", mk(7), mk(3))
		require.NoError(t, err)
		require.Equal(t, "SELECT 1 LIMIT 7 OFFSET 3", got)
	})

	t.Run("caps limit at maximum", func(t *testing.T) {
		got, err := h.applyLimitOffset("SELECT 1", mk(999_999), nil)
		require.NoError(t, err)
		require.Contains(t, got, "LIMIT 10000")
	})

	t.Run("rejects non-positive limit and negative offset", func(t *testing.T) {
		_, err := h.applyLimitOffset("SELECT 1", mk(0), nil)
		require.Error(t, err, "expected error for zero limit")
		_, err = h.applyLimitOffset("SELECT 1", mk(-1), nil)
		require.Error(t, err, "expected error for negative limit")
		_, err = h.applyLimitOffset("SELECT 1", nil, mk(-1))
		require.Error(t, err, "expected error for negative offset")
	})
}

func TestQueryResultToMCP(t *testing.T) {
	t.Run("renders rows as objects keyed by column", func(t *testing.T) {
		res, err := queryResultToMCP(&db.QueryResult{
			Columns: []string{"id", "name"},
			Rows: [][]any{
				{int64(1), "alice"},
				{int64(2), "bob"},
			},
		})
		require.NoError(t, err)
		text := textOf(t, res)
		var payload struct {
			Rows []map[string]any `json:"rows"`
		}
		require.NoError(t, json.Unmarshal([]byte(text), &payload), "payload=%s", text)
		require.Len(t, payload.Rows, 2)
		require.Equal(t, "alice", payload.Rows[0]["name"])
	})

	t.Run("emits empty rows array when result is empty", func(t *testing.T) {
		res, err := queryResultToMCP(&db.QueryResult{Columns: []string{"x"}})
		require.NoError(t, err)
		require.Contains(t, textOf(t, res), `"rows":[]`)
	})
}
