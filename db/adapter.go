package db

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultApplicationName = "cockroachdb-mcp-server"

// Adapter centralizes connection pooling and SQL execution against CockroachDB.
type Adapter struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

// Config is the subset of server configuration the adapter needs to open a
// pool. It is intentionally narrow so the adapter does not depend on the
// config package.
type Config struct {
	DSN          string
	QueryTimeout time.Duration
}

// NewAdapter creates a new pgxpool-backed adapter and verifies connectivity.
func NewAdapter(ctx context.Context, cfg Config) (*Adapter, error) {
	if cfg.DSN == "" {
		return nil, errors.New("DSN cannot be empty")
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, errors.Wrap(err, "parse pool config")
	}
	if _, ok := poolCfg.ConnConfig.RuntimeParams["application_name"]; !ok {
		poolCfg.ConnConfig.RuntimeParams["application_name"] = defaultApplicationName
	}
	// Disable pgx statement caching; stale plans surface as SQLSTATE 26000.
	poolCfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	poolCfg.AfterRelease = func(c *pgx.Conn) bool {
		_, err := c.Exec(context.Background(), "DISCARD ALL")
		return err == nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, errors.Wrap(err, "open pool")
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, errors.Wrap(err, "ping database")
	}

	return &Adapter{
		pool:         pool,
		queryTimeout: cfg.QueryTimeout,
	}, nil
}

// Close releases the underlying connection pool.
func (a *Adapter) Close() {
	if a.pool != nil {
		a.pool.Close()
	}
}

// Query executes a query and returns columns and rows.
func (a *Adapter) Query(ctx context.Context, sql string) (*QueryResult, error) {
	if sql == "" {
		return nil, errors.New("SQL statement cannot be empty")
	}
	if a.queryTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.queryTimeout)
		defer cancel()
	}
	rows, err := a.pool.Query(ctx, sql)
	if err != nil {
		return nil, errors.Wrap(err, "exec query")
	}
	defer rows.Close()
	return scanRows(rows)
}

func scanRows(rows pgx.Rows) (*QueryResult, error) {
	fds := rows.FieldDescriptions()
	columns := make([]string, len(fds))
	for i, fd := range fds {
		columns[i] = string(fd.Name)
	}
	out := &QueryResult{Columns: columns, Rows: make([][]any, 0)}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, errors.Wrap(err, "scan row")
		}
		for i, v := range values {
			if b, ok := v.([]byte); ok {
				values[i] = string(b)
			}
		}
		out.Rows = append(out.Rows, values)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterate rows")
	}
	return out, nil
}
