package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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
		if err != nil {
			t.Fatalf("listDatabases: %v", err)
		}

		if len(fq.queries) != 1 {
			t.Fatalf("expected 1 query, got %d", len(fq.queries))
		}
		got := fq.queries[0]
		if !strings.Contains(got, "SHOW DATABASES") {
			t.Fatalf("query should select from SHOW DATABASES: %q", got)
		}
		if !strings.HasSuffix(got, "LIMIT 100") {
			t.Fatalf("query should append default LIMIT 100: %q", got)
		}

		text := textOf(t, res)
		var payload struct {
			Rows []map[string]any `json:"rows"`
		}
		if err := json.Unmarshal([]byte(text), &payload); err != nil {
			t.Fatalf("unmarshal response: %v\n%s", err, text)
		}
		if len(payload.Rows) != 2 || payload.Rows[0]["database_name"] != "defaultdb" {
			t.Fatalf("unexpected response payload: %v", payload.Rows)
		}
	})

	t.Run("custom limit and offset flow through to the query", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)

		params := ListDatabasesParams{PaginationParams: PaginationParams{
			Limit: mkInt(25), Offset: mkInt(50),
		}}
		if _, _, err := h.listDatabases(context.Background(), &mcp.CallToolRequest{}, params); err != nil {
			t.Fatalf("listDatabases: %v", err)
		}
		if !strings.HasSuffix(fq.queries[0], "LIMIT 25 OFFSET 50") {
			t.Fatalf("query should append custom pagination: %q", fq.queries[0])
		}
	})

	t.Run("limit above MaxRowsCount is capped in the emitted query", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)
		h.cfg.MaxRowsCount = 50

		params := ListDatabasesParams{PaginationParams: PaginationParams{Limit: mkInt(999_999)}}
		if _, _, err := h.listDatabases(context.Background(), &mcp.CallToolRequest{}, params); err != nil {
			t.Fatalf("listDatabases: %v", err)
		}
		if !strings.HasSuffix(fq.queries[0], "LIMIT 50") {
			t.Fatalf("query should cap LIMIT at MaxRowsCount: %q", fq.queries[0])
		}
	})

	t.Run("offset past available rows returns empty rows array", func(t *testing.T) {
		fq := &fakeQuerier{result: &db.QueryResult{Columns: sampleResult.Columns}}
		h := newHandlers(fq)

		params := ListDatabasesParams{PaginationParams: PaginationParams{Offset: mkInt(100)}}
		res, _, err := h.listDatabases(context.Background(), &mcp.CallToolRequest{}, params)
		if err != nil {
			t.Fatalf("listDatabases: %v", err)
		}
		if !strings.HasSuffix(fq.queries[0], "LIMIT 100 OFFSET 100") {
			t.Fatalf("query should include offset: %q", fq.queries[0])
		}
		text := textOf(t, res)
		if !strings.Contains(text, `"rows":[]`) {
			t.Fatalf("expected empty rows array, got %q", text)
		}
	})

	t.Run("negative pagination is rejected before any query is run", func(t *testing.T) {
		fq := &fakeQuerier{result: sampleResult}
		h := newHandlers(fq)

		params := ListDatabasesParams{PaginationParams: PaginationParams{Limit: mkInt(-1)}}
		_, _, err := h.listDatabases(context.Background(), &mcp.CallToolRequest{}, params)
		if err == nil {
			t.Fatal("expected error for negative limit")
		}
		if len(fq.queries) != 0 {
			t.Fatalf("no query should have been issued, got %d", len(fq.queries))
		}
	})

	t.Run("DB errors are wrapped with the tool name", func(t *testing.T) {
		fq := &fakeQuerier{err: errors.New("boom")}
		h := newHandlers(fq)

		_, _, err := h.listDatabases(context.Background(), &mcp.CallToolRequest{}, ListDatabasesParams{})
		if err == nil {
			t.Fatal("expected error from querier")
		}
		if !strings.Contains(err.Error(), "list databases") {
			t.Fatalf("error should be wrapped with 'list databases': %v", err)
		}
		if !strings.Contains(err.Error(), "boom") {
			t.Fatalf("error should retain the cause: %v", err)
		}
	})
}
