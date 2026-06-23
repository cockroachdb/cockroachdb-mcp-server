package tools

import (
	"context"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// createDatabase handles the create_database tool.
func (h *ToolHandlers) createDatabase(
	ctx context.Context, _ *mcp.CallToolRequest, params CreateDatabaseParams,
) (*mcp.CallToolResult, any, error) {
	if params.Name == "" {
		return nil, nil, errors.New("name is required")
	}

	stmt := "CREATE DATABASE %1"
	if params.IfNotExists {
		stmt = "CREATE DATABASE IF NOT EXISTS %1"
	}
	query, err := db.SafeFormat(stmt, db.Identifier(params.Name))
	if err != nil {
		return nil, nil, err
	}
	if _, err := h.dm.Exec(ctx, query); err != nil {
		return nil, nil, errors.Wrap(err, "create database")
	}
	return writeOK(map[string]any{"database": params.Name})
}
