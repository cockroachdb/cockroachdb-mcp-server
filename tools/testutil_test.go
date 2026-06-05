package tools

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/config"
	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// newHandlers builds a ToolHandlers with a minimal cfg populated.
func newHandlers(dm DBManager) *ToolHandlers {
	return NewToolHandlers(dm, &config.Config{MaxRowsCount: 10000})
}

func textOf(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if res == nil {
		t.Fatal("nil call tool result")
	}
	if len(res.Content) == 0 {
		t.Fatal("no content in result")
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}
	return tc.Text
}

// fakeQuerier is a test double for the DBManager interface that records the
// SQL statements it receives and returns a canned result or error.
type fakeQuerier struct {
	queries []string
	result  *db.QueryResult
	err     error
}

func (f *fakeQuerier) Query(_ context.Context, sql string) (*db.QueryResult, error) {
	f.queries = append(f.queries, sql)
	return f.result, f.err
}
