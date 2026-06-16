package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestListDatabases(t *testing.T) {
	sampleResult := &db.QueryResult{
		Columns: []string{"database_name", "owner"},
		Rows: [][]any{
			{"defaultdb", "root"},
			{"appdb", "root"},
		},
	}

	mkInt := func(v int64) *int64 { return &v }

	t.Run("default pagination issues query with default limit", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)

		res, _, err := h.listDatabases(context.Background(), &mcp.CallToolRequest{}, ListDatabasesParams{})
		require.NoError(t, err)

		require.Len(t, fq.queries, 1)
		got := fq.queries[0]
		require.Contains(t, got, "SHOW DATABASES")
		require.Truef(t, strings.HasSuffix(got, "LIMIT 100"),
			"query should append default LIMIT 100: %q", got)

		text := textOf(t, res)
		var payload struct {
			Rows []map[string]any `json:"rows"`
		}
		require.NoError(t, json.Unmarshal([]byte(text), &payload), "payload=%s", text)
		require.Len(t, payload.Rows, 2)
		require.Equal(t, "defaultdb", payload.Rows[0]["database_name"])
	})

	t.Run("custom limit and offset flow through to the query", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)

		params := ListDatabasesParams{PaginationParams: PaginationParams{
			Limit: mkInt(25), Offset: mkInt(50),
		}}
		_, _, err := h.listDatabases(context.Background(), &mcp.CallToolRequest{}, params)
		require.NoError(t, err)
		require.Truef(t, strings.HasSuffix(fq.queries[0], "LIMIT 25 OFFSET 50"),
			"query should append custom pagination: %q", fq.queries[0])
	})

	t.Run("limit above MaxRowsCount is capped in the emitted query", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)
		h.cfg.MaxRowsCount = 50

		params := ListDatabasesParams{PaginationParams: PaginationParams{Limit: mkInt(999_999)}}
		_, _, err := h.listDatabases(context.Background(), &mcp.CallToolRequest{}, params)
		require.NoError(t, err)
		require.Truef(t, strings.HasSuffix(fq.queries[0], "LIMIT 50"),
			"query should cap LIMIT at MaxRowsCount: %q", fq.queries[0])
	})

	t.Run("offset past available rows returns empty rows array", func(t *testing.T) {
		fq := &fakeQuerier{result: &db.QueryResult{Columns: sampleResult.Columns}}
		h := newHandlers(fq)

		params := ListDatabasesParams{PaginationParams: PaginationParams{Offset: mkInt(100)}}
		res, _, err := h.listDatabases(context.Background(), &mcp.CallToolRequest{}, params)
		require.NoError(t, err)
		require.Truef(t, strings.HasSuffix(fq.queries[0], "LIMIT 100 OFFSET 100"),
			"query should include offset: %q", fq.queries[0])
		require.Contains(t, textOf(t, res), `"rows":[]`)
	})

	t.Run("negative pagination is rejected before any query is run", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)

		params := ListDatabasesParams{PaginationParams: PaginationParams{Limit: mkInt(-1)}}
		_, _, err := h.listDatabases(context.Background(), &mcp.CallToolRequest{}, params)
		require.Error(t, err, "expected error for negative limit")
		require.Empty(t, fq.queries, "no query should have been issued")
	})

	t.Run("DB errors are wrapped with the tool name", func(t *testing.T) {
		fq := &fakeQuerier{err: errors.New("boom")}
		h := newHandlers(fq)

		_, _, err := h.listDatabases(context.Background(), &mcp.CallToolRequest{}, ListDatabasesParams{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "list databases")
		require.Contains(t, err.Error(), "boom")
	})
}
