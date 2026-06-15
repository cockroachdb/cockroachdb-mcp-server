package tools

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestCreateDatabase(t *testing.T) {
	t.Run("quotes the database identifier", func(t *testing.T) {
		fq := &fakeQuerier{}
		h := newWriteHandlers(fq)
		_, _, err := h.createDatabase(context.Background(), &mcp.CallToolRequest{}, CreateDatabaseParams{Name: "appdb"})
		require.NoError(t, err)
		require.Equal(t, `CREATE DATABASE "appdb"`, fq.execs[0])
	})

	t.Run("escapes embedded quotes in identifier", func(t *testing.T) {
		fq := &fakeQuerier{}
		h := newWriteHandlers(fq)
		_, _, err := h.createDatabase(context.Background(), &mcp.CallToolRequest{}, CreateDatabaseParams{Name: `na"me`})
		require.NoError(t, err)
		require.Equal(t, `CREATE DATABASE "na""me"`, fq.execs[0])
	})

	t.Run("missing name is rejected", func(t *testing.T) {
		fq := &fakeQuerier{}
		h := newWriteHandlers(fq)
		_, _, err := h.createDatabase(context.Background(), &mcp.CallToolRequest{}, CreateDatabaseParams{})
		require.Error(t, err, "expected error for empty name")
		require.Empty(t, fq.execs, "no exec should have been issued")
	})

	t.Run("emits IF NOT EXISTS when requested", func(t *testing.T) {
		fq := &fakeQuerier{}
		h := newWriteHandlers(fq)
		_, _, err := h.createDatabase(context.Background(), &mcp.CallToolRequest{},
			CreateDatabaseParams{Name: "appdb", IfNotExists: true})
		require.NoError(t, err)
		require.Equal(t, `CREATE DATABASE IF NOT EXISTS "appdb"`, fq.execs[0])
	})
}
