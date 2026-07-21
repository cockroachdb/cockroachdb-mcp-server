package tools

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestRegisterToolsGating(t *testing.T) {
	readTools := []string{
		"list_databases", "list_tables", "get_table_schema", "get_cluster",
		"list_sql_users", "list_cluster_nodes", "show_running_queries",
		"select_query", "explain_query", "show_statement",
	}
	writeTools := []string{"create_database", "create_table", "insert_rows", "update_rows", "delete_rows"}

	listTools := func(t *testing.T, enableWrites bool) map[string]*mcp.Tool {
		t.Helper()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
		cfg := &config.Config{MaxRowsCount: 10000, EnableWriteQueries: enableWrites}
		NewToolHandlers(&fakeQuerier{}, cfg).RegisterTools(server)

		cTransport, sTransport := mcp.NewInMemoryTransports()
		ss, err := server.Connect(ctx, sTransport, nil)
		require.NoError(t, err)
		t.Cleanup(func() { _ = ss.Close() })

		cs, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil).Connect(ctx, cTransport, nil)
		require.NoError(t, err)
		t.Cleanup(func() { _ = cs.Close() })

		res, err := cs.ListTools(ctx, nil)
		require.NoError(t, err)
		registered := make(map[string]*mcp.Tool, len(res.Tools))
		for _, tool := range res.Tools {
			registered[tool.Name] = tool
		}
		return registered
	}

	t.Run("write tools absent when EnableWriteQueries=false", func(t *testing.T) {
		registered := listTools(t, false)
		for _, name := range writeTools {
			require.NotContainsf(t, registered, name, "write tool %q should not be registered", name)
		}
	})

	t.Run("write tools present when EnableWriteQueries=true", func(t *testing.T) {
		registered := listTools(t, true)
		for _, name := range writeTools {
			require.Containsf(t, registered, name, "write tool %q should be registered", name)
		}
	})

	t.Run("read tools annotated read-only and closed-world", func(t *testing.T) {
		registered := listTools(t, true)
		for _, name := range readTools {
			require.Containsf(t, registered, name, "read tool %q should be registered", name)
			a := registered[name].Annotations
			require.NotNilf(t, a, "read tool %q should carry annotations", name)
			require.Truef(t, a.ReadOnlyHint, "read tool %q should hint read-only", name)
			require.NotNilf(t, a.OpenWorldHint, "read tool %q should set openWorldHint", name)
			require.Falsef(t, *a.OpenWorldHint, "read tool %q should hint closed-world", name)
		}
	})

	t.Run("write tools carry full annotations", func(t *testing.T) {
		registered := listTools(t, true)
		for _, name := range writeTools {
			a := registered[name].Annotations
			require.NotNilf(t, a, "write tool %q should carry annotations", name)
			require.Falsef(t, a.ReadOnlyHint, "write tool %q must not hint read-only", name)
			require.NotNilf(t, a.OpenWorldHint, "write tool %q should set openWorldHint", name)
			require.Falsef(t, *a.OpenWorldHint, "write tool %q should hint closed-world", name)
			require.NotNilf(t, a.DestructiveHint, "write tool %q should set destructiveHint", name)
		}
	})

	t.Run("destructive hints match tool semantics", func(t *testing.T) {
		registered := listTools(t, true)
		for _, tc := range []struct {
			name        string
			destructive bool
		}{
			{"create_database", false},
			{"create_table", false},
			{"insert_rows", true},
			{"update_rows", true},
			{"delete_rows", true},
		} {
			a := registered[tc.name].Annotations
			require.Equalf(t, tc.destructive, *a.DestructiveHint, "tool %q destructiveHint", tc.name)
		}
	})

	t.Run("idempotent hints match tool semantics", func(t *testing.T) {
		registered := listTools(t, true)
		for _, tc := range []struct {
			name       string
			idempotent bool
		}{
			{"create_database", false},
			{"create_table", false},
			{"insert_rows", false},
			{"update_rows", false},
			{"delete_rows", true},
		} {
			a := registered[tc.name].Annotations
			require.Equalf(t, tc.idempotent, a.IdempotentHint, "tool %q idempotentHint", tc.name)
		}
	})
}
