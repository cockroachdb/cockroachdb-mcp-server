// Package db provides SQL execution against CockroachDB for the MCP server.
package db

import (
	"context"

	"github.com/cockroachdb/cockroachdb-mcp-server/config"
)

// Manager is the high-level facade used by MCP tool handlers.
type Manager struct {
	adapter *Adapter
}

// NewManager opens an adapter using the provided server config.
func NewManager(ctx context.Context, cfg *config.Config) (*Manager, error) {
	adapter, err := NewAdapter(ctx, Config{
		DSN:          cfg.DSN(),
		QueryTimeout: cfg.QueryTimeout,
	})
	if err != nil {
		return nil, err
	}
	return &Manager{adapter: adapter}, nil
}

// Close releases the underlying pool.
func (m *Manager) Close() {
	if m.adapter != nil {
		m.adapter.Close()
	}
}

// QueryResult is the projection of a query result returned to tool handlers.
type QueryResult struct {
	Columns []string
	Rows    [][]any
}

// Query runs a query against the bootstrap database.
func (m *Manager) Query(ctx context.Context, sql string) (*QueryResult, error) {
	return m.adapter.Query(ctx, sql)
}
