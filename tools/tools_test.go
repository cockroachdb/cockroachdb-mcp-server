package tools

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestRegisterToolsGating(t *testing.T) {
	writeTools := []string{"create_database", "create_table", "insert_rows"}

	listToolNames := func(t *testing.T, enableWrites bool) map[string]bool {
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
		registered := make(map[string]bool, len(res.Tools))
		for _, tool := range res.Tools {
			registered[tool.Name] = true
		}
		return registered
	}

	t.Run("write tools absent when EnableWriteQueries=false", func(t *testing.T) {
		registered := listToolNames(t, false)
		for _, name := range writeTools {
			require.Falsef(t, registered[name], "write tool %q should not be registered", name)
		}
	})

	t.Run("write tools present when EnableWriteQueries=true", func(t *testing.T) {
		registered := listToolNames(t, true)
		for _, name := range writeTools {
			require.Truef(t, registered[name], "write tool %q should be registered", name)
		}
	})
}
