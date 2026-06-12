package tools

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestListSQLUsers(t *testing.T) {
	fq := &fakeQuerier{result: &db.QueryResult{Columns: []string{"username"}}}
	h := newHandlers(fq)
	_, _, err := h.listSQLUsers(context.Background(), &mcp.CallToolRequest{}, ListSQLUsersParams{})
	require.NoError(t, err)
	require.Contains(t, fq.queries[0], "SHOW USERS")
}
