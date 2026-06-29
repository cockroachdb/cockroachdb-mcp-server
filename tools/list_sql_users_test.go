package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestListSQLUsers(t *testing.T) {
	t.Run("wraps SHOW USERS in a SELECT so pagination is valid SQL", func(t *testing.T) {
		fq := &fakeQuerier{result: &db.QueryResult{Columns: []string{"username"}}}
		h := newHandlers(fq)

		_, _, err := h.listSQLUsers(context.Background(), &mcp.CallToolRequest{}, ListSQLUsersParams{})
		require.NoError(t, err)

		require.Len(t, fq.queries, 1)
		require.Truef(t, strings.HasPrefix(fq.queries[0], "SELECT * FROM [SHOW USERS]"),
			"SHOW USERS must be bracket-wrapped so LIMIT/OFFSET attach to a SELECT: %q",
			fq.queries[0])
	})

	t.Run("DB errors are wrapped with the tool name", func(t *testing.T) {
		fq := &fakeQuerier{err: errors.New("boom")}
		h := newHandlers(fq)

		_, _, err := h.listSQLUsers(context.Background(), &mcp.CallToolRequest{}, ListSQLUsersParams{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "list sql users")
		require.Contains(t, err.Error(), "boom")
	})
}
