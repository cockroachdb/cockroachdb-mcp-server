package tools

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/config"
	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// newHandlers builds a ToolHandlers with a minimal cfg populated.
func newHandlers(dm DBManager) *ToolHandlers {
	return NewToolHandlers(dm, &config.Config{MaxRowsCount: 10000})
}

// newWriteHandlers is newHandlers with write tools enabled.
func newWriteHandlers(dm DBManager) *ToolHandlers {
	return NewToolHandlers(dm, &config.Config{MaxRowsCount: 10000, EnableWriteQueries: true})
}

// rejectionCase is a table entry asserting a write tool rejects a statement
// before it reaches the database.
type rejectionCase struct {
	name, statement, wantContains string
}

func textOf(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	require.NotNil(t, res, "nil call tool result")
	require.NotEmpty(t, res.Content, "no content in result")
	require.IsType(t, &mcp.TextContent{}, res.Content[0])
	return res.Content[0].(*mcp.TextContent).Text
}

// fakeQuerier is a test double for the DBManager interface that records the
// SQL statements it receives and returns a canned result or error.
type fakeQuerier struct {
	queries      []string
	execs        []string
	result       *db.QueryResult
	rowsAffected int64
	err          error
}

func (f *fakeQuerier) Query(_ context.Context, sql string) (*db.QueryResult, error) {
	f.queries = append(f.queries, sql)
	return f.result, f.err
}

func (f *fakeQuerier) Exec(_ context.Context, sql string) (int64, error) {
	f.execs = append(f.execs, sql)
	return f.rowsAffected, f.err
}

// scriptedStep is one canned Query response for scriptedQuerier.
type scriptedStep struct {
	result *db.QueryResult
	err    error
}

// scriptedQuerier is a DBManager test double that replays a fixed sequence
// of Query responses, one per call, recording the SQL it receives.
type scriptedQuerier struct {
	queries []string
	steps   []scriptedStep
}

func (s *scriptedQuerier) Query(_ context.Context, sql string) (*db.QueryResult, error) {
	i := len(s.queries)
	s.queries = append(s.queries, sql)
	if i >= len(s.steps) {
		return nil, errors.Newf("unexpected query %d: %s", i, sql)
	}
	return s.steps[i].result, s.steps[i].err
}

func (s *scriptedQuerier) Exec(context.Context, string) (int64, error) {
	return 0, errors.New("unexpected exec")
}
