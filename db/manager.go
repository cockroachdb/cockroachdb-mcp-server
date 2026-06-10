package db

import (
	"context"

	"github.com/cockroachdb/cockroachdb-mcp-server/config"
)

// DBManager is the high-level facade used by MCP tool handlers.
type DBManager struct {
	adapter *Adapter
}

// NewDBManager opens an adapter using the provided server config.
func NewDBManager(ctx context.Context, cfg *config.Config) (*DBManager, error) {
	adapter, err := NewAdapter(ctx, Config{
		DSN:          cfg.DSN(),
		QueryTimeout: cfg.QueryTimeout,
	})
	if err != nil {
		return nil, err
	}
	return &DBManager{adapter: adapter}, nil
}

// Close releases the underlying pool.
func (m *DBManager) Close() {
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
func (m *DBManager) Query(ctx context.Context, sql string) (*QueryResult, error) {
	return m.adapter.Query(ctx, sql)
}
