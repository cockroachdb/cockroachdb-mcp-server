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

	t.Run("read tools annotated read-only", func(t *testing.T) {
		registered := listTools(t, true)
		for _, name := range readTools {
			require.Containsf(t, registered, name, "read tool %q should be registered", name)
			require.NotNilf(t, registered[name].Annotations, "read tool %q should carry annotations", name)
			require.Truef(t, registered[name].Annotations.ReadOnlyHint, "read tool %q should hint read-only", name)
		}
	})

	t.Run("write tools never annotated read-only", func(t *testing.T) {
		registered := listTools(t, true)
		for _, name := range writeTools {
			if a := registered[name].Annotations; a != nil {
				require.Falsef(t, a.ReadOnlyHint, "write tool %q must not hint read-only", name)
			}
		}
	})

	t.Run("row mutation tools annotated destructive", func(t *testing.T) {
		registered := listTools(t, true)
		for _, name := range []string{"update_rows", "delete_rows"} {
			a := registered[name].Annotations
			require.NotNilf(t, a, "tool %q should carry annotations", name)
			require.NotNilf(t, a.DestructiveHint, "tool %q should set a destructive hint", name)
			require.Truef(t, *a.DestructiveHint, "tool %q should hint destructive", name)
		}
	})
}
