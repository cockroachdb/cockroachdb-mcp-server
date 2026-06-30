# CockroachDB MCP Server

A [Model Context Protocol](https://modelcontextprotocol.io) server that exposes
CockroachDB to AI agents as a set of typed tools.

## Project structure

```
cockroachdb-mcp-server/
├── main.go        # stdio + http entrypoint, --version, graceful shutdown
├── config/        # env-driven configuration and DSN builder
├── db/            # pgx pool and SQL execution
├── auth/          # bearer-token HTTP middleware
└── tools/         # MCP tool handlers and JSON input schemas
```

## Prerequisites

- Go 1.25+
- A reachable CockroachDB cluster (local or CockroachCloud)

## Build

```bash
go build -o bin/cockroachdb-mcp-server .
./bin/cockroachdb-mcp-server --version
```

> Pre-built binaries cover `linux/{amd64,arm64}` and `windows/{amd64,arm64}`. macOS users: use the Docker image (runs natively on Apple Silicon) or `go install github.com/cockroachdb/cockroachdb-mcp-server@latest`.

## Configuration

All configuration is via environment variables. `sslmode` must be `require`,
`verify-ca`, or `verify-full` in both auth modes.

**Auth - choose one:**

| Variable | Purpose |
| --- | --- |
| `CRDB_DATABASE_URL` | Full libpq connection string (preferred when set) |

Or the cert-based vars:

| Variable | Purpose | Default |
| --- | --- | --- |
| `CRDB_HOST` | Hostname | required |
| `CRDB_PORT` | Port | `26257` |
| `CRDB_USERNAME` | SQL user | required |
| `CRDB_PWD` | Password (discouraged, see note below) | - |
| `CRDB_SSL_MODE` | `require`, `verify-ca`, or `verify-full` | `verify-full` |
| `CRDB_SSL_CA_PATH` | CA cert path (required for `verify-ca` / `verify-full`) | - |
| `CRDB_SSL_CERTFILE` | Client cert path | required |
| `CRDB_SSL_KEYFILE` | Client key path | required |

> **Cert-based auth is recommended in stdio mode.** The server runs as a
> subprocess of the AI agent host, which can read `CRDB_PWD`, a password
> embedded in `CRDB_DATABASE_URL`, `PGPASSWORD`, or `~/.pgpass` from this
> process's environment. To protect those credentials, password-based auth is
> rejected by default. Set `CRDB_MCP_ALLOW_PASSWORD_AUTH=true` to opt in.

**Behaviour:**

| Variable | Purpose | Default |
| --- | --- | --- |
| `CRDB_MCP_QUERY_TIMEOUT` | Per-query timeout (Go duration, e.g. `30s`) | `30s` |
| `CRDB_MCP_MAX_ROWS_COUNT` | Caps the max LIMIT a list-style tool will issue to CRDB. Must be a positive integer. | `10000` |
| `CRDB_MCP_ENABLE_WRITE_QUERIES` | Gates the write tools (`create_database`, `create_table`, `insert_rows`) that land in a follow-up PR. `false` keeps the server read-only | `false` |
| `CRDB_MCP_ALLOW_PASSWORD_AUTH` | Opt-in to password-based auth (rejected by default) | `false` |
| `CRDB_MCP_MAX_CONNS` | Upper bound on the pgxpool connection count. Must be a positive integer ≤ 100. | `10` |
| `CRDB_MCP_TXN_QOS` | Default transaction QoS for MCP sessions. One of `background`, `regular`, `critical`. | `background` |
| `CRDB_MCP_TRANSPORT` | Transport to serve MCP on. `stdio` or `http` | `stdio` |
| `CRDB_MCP_HTTP_LISTEN_ADDR` | Listen address when `CRDB_MCP_TRANSPORT=http` | `:8080` |
| `CRDB_MCP_BEARER_TOKEN` | Bearer token clients must present in `Authorization: Bearer <token>`. Required when `CRDB_MCP_TRANSPORT=http`; must be at least 16 characters | - |
| `CRDB_MCP_TLS_CERT` | PEM-encoded server certificate path. Required for HTTPS unless `CRDB_MCP_ALLOW_INSECURE_HTTP=true` | - |
| `CRDB_MCP_TLS_KEY` | PEM-encoded private key path. Required for HTTPS unless `CRDB_MCP_ALLOW_INSECURE_HTTP=true` | - |
| `CRDB_MCP_ALLOW_INSECURE_HTTP` | Explicit opt-in to run HTTP mode without TLS (cleartext). Intended for deployments behind a TLS-terminating reverse proxy | `false` |

The server sets `default_transaction_quality_of_service=background` on every
session so MCP traffic does not contend with latency-sensitive foreground
workloads. Precedence for picking the value:

1. `CRDB_MCP_TXN_QOS` if set (`background`, `regular`, or `critical`).
2. Otherwise, a `default_transaction_quality_of_service=...` query param in
   `CRDB_DATABASE_URL`, if present.
3. Otherwise, `background`.

## Run

Default is stdio:

```bash
export CRDB_DATABASE_URL="postgresql://user@host:26257/defaultdb?sslmode=verify-full&sslcert=/path/client.crt&sslkey=/path/client.key&sslrootcert=/path/ca.crt"
./bin/cockroachdb-mcp-server
```

Or as an HTTPS service with bearer-token auth:

```bash
export CRDB_DATABASE_URL="postgresql://..."
export CRDB_MCP_TRANSPORT=http
export CRDB_MCP_HTTP_LISTEN_ADDR=:8443
export CRDB_MCP_BEARER_TOKEN="$(openssl rand -hex 32)"
export CRDB_MCP_TLS_CERT=/etc/mcp/tls.crt
export CRDB_MCP_TLS_KEY=/etc/mcp/tls.key
./bin/cockroachdb-mcp-server
```

If you terminate TLS at a trusted reverse proxy, opt into cleartext HTTP explicitly:

```bash
export CRDB_MCP_TRANSPORT=http
export CRDB_MCP_BEARER_TOKEN="$(openssl rand -hex 32)"
export CRDB_MCP_ALLOW_INSECURE_HTTP=true
./bin/cockroachdb-mcp-server
```

> **HTTP mode is TLS-by-default.** Starting without `CRDB_MCP_TLS_CERT` and
> `CRDB_MCP_TLS_KEY` is rejected unless `CRDB_MCP_ALLOW_INSECURE_HTTP=true` is
> set, in which case a startup warning is logged. `/healthz` and `/ready` are
> unauthenticated and `GET`/`HEAD`-only for orchestrator probes; all other
> paths require the bearer token.

### Tools shipped today

| Tool | Description |
| --- | --- |
| `list_databases` | List all databases in the CockroachDB cluster. Accepts optional `limit` (default 100, max 10000) and `offset`. |
| `list_tables` | List tables in a database. Required: `database`. Optional: `limit`, `offset`. |
| `get_table_schema` | Return the `CREATE TABLE` statement for a table. Required: `database`, `table`. Optional: `schema` (defaults to `public`). |
| `show_running_queries` | List currently executing CockroachDB cluster statements, ordered by start time descending. Optional `limit` (default 100, max 10000) and `offset`. |
| `get_cluster` | Return CockroachDB cluster identity and version metadata: cluster_id, cluster_name, binary_version, active_version. |
| `list_sql_users` | List SQL users defined in the CockroachDB cluster. Optional `limit` (default 100, max 10000) and `offset`. |
| `list_cluster_nodes` | List CockroachDB cluster nodes with address, liveness, and locality. Requires admin or VIEWCLUSTERMETADATA on the connecting role. |
| `select_query` | Execute a single agent-supplied SELECT (parser-validated). A default LIMIT of 100 is appended when none is supplied; the cap is `CRDB_MCP_MAX_ROWS_COUNT`. |
| `explain_query` | Return the EXPLAIN plan for an agent-supplied SQL statement (parser-validated) without executing it. `EXPLAIN ANALYZE` (and `EXPLAIN ANALYZE (DEBUG)`) is rejected. `EXPLAIN` with display options (`VERBOSE`, `DISTSQL`, `TYPES`, `OPT`, etc.) is passed through. |
| `show_statement` | Execute an agent-supplied SHOW statement (parser-validated) such as `SHOW SCHEMAS`, `SHOW INDEXES`, `SHOW REGIONS`. Optional `limit` (default 100, max 10000) and `offset`. |
| `create_database` | Create a database. Required: `name`. Requires `CRDB_MCP_ENABLE_WRITE_QUERIES=true`. |
| `create_table` | Execute a single `CREATE TABLE` statement (parser-validated). Required: `statement`. Requires `CRDB_MCP_ENABLE_WRITE_QUERIES=true`. |
| `insert_rows` | Execute a single `INSERT` statement (parser-validated) and return rows affected. Required: `statement`. Requires `CRDB_MCP_ENABLE_WRITE_QUERIES=true`. |

Grant the connecting SQL role only the privileges the registered tools need. Avoid admin and write privileges unless write tools are explicitly registered.

Additional read and write tools land in follow-up PRs.

## Test

```bash
go test ./...
```

### With MCP Inspector

```bash
npm install -g @modelcontextprotocol/inspector
mcp-inspector ./bin/cockroachdb-mcp-server
```

## License

[Apache License 2.0](LICENSE)
