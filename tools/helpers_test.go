package tools

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
)

func TestApplyLimitOffset(t *testing.T) {
	mk := func(v int64) *int64 { return &v }
	h := newHandlers(nil)

	t.Run("uses default when limit is nil", func(t *testing.T) {
		got, err := h.applyLimitOffset("SELECT 1", nil, nil, 25)
		if err != nil {
			t.Fatalf("applyLimitOffset: %v", err)
		}
		if got != "SELECT 1 LIMIT 25" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("respects explicit limit", func(t *testing.T) {
		got, err := h.applyLimitOffset("SELECT 1", mk(7), nil, 25)
		if err != nil {
			t.Fatalf("applyLimitOffset: %v", err)
		}
		if got != "SELECT 1 LIMIT 7" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("appends offset", func(t *testing.T) {
		got, err := h.applyLimitOffset("SELECT 1", mk(7), mk(3), 25)
		if err != nil {
			t.Fatalf("applyLimitOffset: %v", err)
		}
		if got != "SELECT 1 LIMIT 7 OFFSET 3" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("caps limit at maximum", func(t *testing.T) {
		got, err := h.applyLimitOffset("SELECT 1", mk(999_999), nil, 25)
		if err != nil {
			t.Fatalf("applyLimitOffset: %v", err)
		}
		if !strings.Contains(got, "LIMIT 10000") {
			t.Fatalf("expected limit cap, got %q", got)
		}
	})

	t.Run("rejects non-positive limit and negative offset", func(t *testing.T) {
		if _, err := h.applyLimitOffset("SELECT 1", mk(0), nil, 25); err == nil {
			t.Fatal("expected error for zero limit")
		}
		if _, err := h.applyLimitOffset("SELECT 1", mk(-1), nil, 25); err == nil {
			t.Fatal("expected error for negative limit")
		}
		if _, err := h.applyLimitOffset("SELECT 1", nil, mk(-1), 25); err == nil {
			t.Fatal("expected error for negative offset")
		}
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
		if err != nil {
			t.Fatalf("queryResultToMCP: %v", err)
		}
		text := textOf(t, res)
		var payload struct {
			Rows []map[string]any `json:"rows"`
		}
		if err := json.Unmarshal([]byte(text), &payload); err != nil {
			t.Fatalf("unmarshal: %v\npayload=%s", err, text)
		}
		if len(payload.Rows) != 2 {
			t.Fatalf("expected 2 rows, got %d", len(payload.Rows))
		}
		if payload.Rows[0]["name"] != "alice" {
			t.Fatalf("unexpected first row: %v", payload.Rows[0])
		}
	})

	t.Run("emits empty rows array when result is empty", func(t *testing.T) {
		res, err := queryResultToMCP(&db.QueryResult{Columns: []string{"x"}})
		if err != nil {
			t.Fatalf("queryResultToMCP: %v", err)
		}
		text := textOf(t, res)
		if !strings.Contains(text, `"rows":[]`) {
			t.Fatalf("expected empty rows array, got %q", text)
		}
	})
}
