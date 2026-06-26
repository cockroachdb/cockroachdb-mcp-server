package db

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultApplicationName = "cockroachdb-mcp-server"
	// defaultTxnQoS is applied when neither cfg.TxnQoS nor the DSN specifies
	// one. "background" tells CRDB's admission control to yield to
	// latency-sensitive foreground SQL.
	defaultTxnQoS = "background"
)

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
	// ReadOnly forces every pool session into read-only mode at SQL layer.
	ReadOnly bool
	// AllowPasswordAuth permits password-based connections when true; rejected
	// by default to keep credentials out of the agent host environment.
	AllowPasswordAuth bool
	MaxConns          int32
	TxnQoS            string
}

// NewAdapter creates a new pgxpool-backed adapter and verifies connectivity.
func NewAdapter(ctx context.Context, cfg Config) (*Adapter, error) {
	poolCfg, err := buildPoolConfig(cfg)
	if err != nil {
		return nil, err
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

// buildPoolConfig is the pure half of NewAdapter: it does no I/O, so the
// defaults applied here are unit-testable without a live database.
func buildPoolConfig(cfg Config) (*pgxpool.Config, error) {
	if cfg.DSN == "" {
		return nil, errors.New("DSN cannot be empty")
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, errors.Wrap(err, "parse pool config")
	}
	if poolCfg.ConnConfig.Password != "" && !cfg.AllowPasswordAuth {
		return nil, errors.New(
			"password-based auth is disabled; set CRDB_MCP_ALLOW_PASSWORD_AUTH=true to enable, or use cert-based auth",
		)
	}
	if _, ok := poolCfg.ConnConfig.RuntimeParams["application_name"]; !ok {
		poolCfg.ConnConfig.RuntimeParams["application_name"] = defaultApplicationName
	}
	// QoS precedence: explicit cfg (env var) wins; otherwise honor a
	// DSN-supplied value; otherwise apply background default. The value lands
	// in startup params, so CRDB's RESET ALL / DISCARD ALL semantics restore
	// it across pool reuse without an explicit re-apply.
	const qosKey = "default_transaction_quality_of_service"
	switch {
	case cfg.TxnQoS != "":
		poolCfg.ConnConfig.RuntimeParams[qosKey] = cfg.TxnQoS
	case poolCfg.ConnConfig.RuntimeParams[qosKey] == "":
		poolCfg.ConnConfig.RuntimeParams[qosKey] = defaultTxnQoS
	}
	if cfg.MaxConns > 0 {
		poolCfg.MaxConns = cfg.MaxConns
	}
	// Disable pgx statement caching; stale plans surface as SQLSTATE 26000.
	poolCfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec

	// Default: DISCARD ALL after every release to keep sessions clean.
	poolCfg.AfterRelease = func(c *pgx.Conn) bool {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := c.Exec(ctx, "DISCARD ALL")
		return err == nil
	}

	// Read-only is applied via SET (not a startup param), so DISCARD ALL
	// resets it; re-apply on every release.
	if cfg.ReadOnly {
		const setReadOnly = "SET default_transaction_read_only = true"
		poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
			_, err := conn.Exec(ctx, setReadOnly)
			return err
		}
		poolCfg.AfterRelease = func(c *pgx.Conn) bool {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err := c.Exec(ctx, "DISCARD ALL; "+setReadOnly)
			return err == nil
		}
	}

	return poolCfg, nil
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

// Exec runs a non-result-returning statement (DDL/DML) and returns the
// number of rows affected.
func (a *Adapter) Exec(ctx context.Context, sql string) (int64, error) {
	if sql == "" {
		return 0, errors.New("SQL statement cannot be empty")
	}
	if a.queryTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.queryTimeout)
		defer cancel()
	}
	tag, err := a.pool.Exec(ctx, sql)
	if err != nil {
		return 0, errors.Wrap(err, "exec statement")
	}
	return tag.RowsAffected(), nil
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
