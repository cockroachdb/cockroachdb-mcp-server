# CockroachDB MCP Server

A [Model Context Protocol](https://modelcontextprotocol.io) server that exposes
CockroachDB to AI agents as a set of typed tools.

## Project structure

```
cockroachdb-mcp-server/
├── main.go        # stdio entrypoint, --version, graceful shutdown
├── config/        # env-driven configuration and DSN builder
├── db/            # pgx pool and SQL execution
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
| `CRDB_PWD` | Password (optional) | - |
| `CRDB_SSL_MODE` | `require`, `verify-ca`, or `verify-full` | `verify-full` |
| `CRDB_SSL_CA_PATH` | CA cert path (required for `verify-ca` / `verify-full`) | - |
| `CRDB_SSL_CERTFILE` | Client cert path | required |
| `CRDB_SSL_KEYFILE` | Client key path | required |

**Behaviour:**

| Variable | Purpose | Default |
| --- | --- | --- |
| `CRDB_MCP_QUERY_TIMEOUT` | Per-query timeout (Go duration, e.g. `30s`) | `30s` |
| `CRDB_MCP_MAX_ROWS_COUNT` | Caps the max LIMIT a list-style tool will issue to CRDB. Must be a positive integer. | `10000` |
| `CRDB_MCP_ENABLE_WRITE_QUERIES` | Gates the write tools (`create_database`, `create_table`, `insert_rows`) that land in a follow-up PR. `false` keeps the server read-only | `false` |

## Run

```bash
export CRDB_DATABASE_URL="postgresql://user:pass@host:26257/defaultdb?sslmode=verify-full"
./bin/cockroachdb-mcp-server
```

### Tools shipped today

| Tool | Description |
| --- | --- |
| `list_databases` | List all databases in the cluster. Accepts optional `limit` (default 100, max 10000) and `offset`. |

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
