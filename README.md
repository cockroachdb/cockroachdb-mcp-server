<p align="center">
  <img src='docs/media/cockroachdb.png?raw=true' alt='CockroachDB' width='70%'>
</p>

# CockroachDB MCP Server

CockroachDB MCP Server is a [Model Context Protocol](https://modelcontextprotocol.io)
server that exposes CockroachDB to AI agents as a set of typed tools. Ships read
tools by default; write and DDL tools opt in via env var.

- [Install](#install)
- [Setup - MCP client config](#setup---mcp-client-config)
- [Configuration](#configuration)
- [Tools](#tools)
- [Testing](#testing)
- [Contributing](#contributing)
- [Security](#security)
- [License](#license)

## Install

Requires Go 1.26+ and a reachable CockroachDB cluster.

```bash
go install github.com/cockroachdb/cockroachdb-mcp-server@latest
```

The binary lands in `$(go env GOPATH)/bin/cockroachdb-mcp-server`.

### Pre-built binaries

Grab a tarball for `linux/{amd64,arm64}` or `windows/{amd64,arm64}` from
[Releases](https://github.com/cockroachdb/cockroachdb-mcp-server/releases),
available from the first release onwards.

### Build from source

```bash
git clone https://github.com/cockroachdb/cockroachdb-mcp-server
cd cockroachdb-mcp-server
go build -o bin/cockroachdb-mcp-server .
./bin/cockroachdb-mcp-server --version
```

macOS: use `go install` or build from source. Pre-built binaries do not cover
darwin because a CockroachDB parser dependency needs cgo on macOS.

## Setup - MCP client config

Drop one of the following into your MCP client's config (Claude Desktop, Cursor,
VS Code, Copilot CLI, etc.). Each client reads an `mcpServers` block or its own
equivalent; see your client's docs for the file location.

> GUI clients (e.g. Claude Desktop) often launch with a minimal PATH and do
> not expand `~`, so a bare `command` may not resolve; use the absolute
> binary path instead, e.g. `/Users/<username>/go/bin/cockroachdb-mcp-server`.

### stdio + cert-based auth (recommended)

```json
{
  "mcpServers": {
    "cockroachdb": {
      "command": "cockroachdb-mcp-server",
      "env": {
        "CRDB_HOST": "my-cluster.crdb.io",
        "CRDB_USERNAME": "ai_agent",
        "CRDB_SSL_MODE": "verify-full",
        "CRDB_SSL_CA_PATH": "/certs/ca.crt",
        "CRDB_SSL_CERTFILE": "/certs/client.ai_agent.crt",
        "CRDB_SSL_KEYFILE": "/certs/client.ai_agent.key"
      }
    }
  }
}
```

### stdio + full connection string

```json
{
  "mcpServers": {
    "cockroachdb": {
      "command": "cockroachdb-mcp-server",
      "env": {
        "CRDB_DATABASE_URL": "postgresql://ai_agent@my-cluster.crdb.io:26257/defaultdb?sslmode=verify-full&sslcert=/certs/client.ai_agent.crt&sslkey=/certs/client.ai_agent.key&sslrootcert=/certs/ca.crt"
      }
    }
  }
}
```

### stdio + local insecure cluster (development only, writes enabled)

For a local `cockroach start-single-node --insecure` or `cockroach demo --insecure`
cluster, opt in to a TLS-free connection; no cert files are needed:

```json
{
  "mcpServers": {
    "cockroachdb": {
      "command": "cockroachdb-mcp-server",
      "env": {
        "CRDB_DATABASE_URL": "postgresql://root@localhost:26257/defaultdb?sslmode=disable",
        "CRDB_MCP_ALLOW_INSECURE_DB": "true",
        "CRDB_MCP_ENABLE_WRITE_QUERIES": "true"
      }
    }
  }
}
```

### HTTPS (shared / remote deployments)

Run the server as a long-lived process, then point your MCP client at the URL
with the bearer token:

```bash
export CRDB_DATABASE_URL="postgresql://..."
export CRDB_MCP_TRANSPORT=http
export CRDB_MCP_HTTP_LISTEN_ADDR=0.0.0.0:8443
export CRDB_MCP_BEARER_TOKEN="$(openssl rand -hex 32)"
export CRDB_MCP_TLS_CERT=/etc/mcp/tls.crt
export CRDB_MCP_TLS_KEY=/etc/mcp/tls.key
cockroachdb-mcp-server
```

Set `CRDB_MCP_HTTP_LISTEN_ADDR` to whatever `host:port` fits your deployment,
for example `127.0.0.1:9090` to serve only local traffic behind a
TLS-terminating reverse proxy (with `CRDB_MCP_ALLOW_INSECURE_HTTP=true`), or
`10.0.0.5:8443` to bind a specific interface.

Then point your MCP client at the server:

```json
{
  "mcpServers": {
    "cockroachdb": {
      "type": "http",
      "url": "https://mcp.example.com:8443",
      "headers": {
        "Authorization": "Bearer <token>"
      }
    }
  }
}
```

> HTTP mode is TLS-by-default. Starting without `CRDB_MCP_TLS_CERT` and
> `CRDB_MCP_TLS_KEY` is rejected unless `CRDB_MCP_ALLOW_INSECURE_HTTP=true`
> (for deployments behind a TLS-terminating reverse proxy). `/healthz` and
> `/ready` are unauthenticated `GET`/`HEAD` probes; all other paths require the
> bearer token.

## Configuration

All configuration is via environment variables.

### Authentication

Cert-based auth is recommended in stdio mode. The server runs as a subprocess
of the AI agent host, which can read `CRDB_PWD`, a password embedded in
`CRDB_DATABASE_URL`, `PGPASSWORD`, or `~/.pgpass` from this process's
environment. To protect those credentials, password-based auth is rejected by
default. Set `CRDB_MCP_ALLOW_PASSWORD_AUTH=true` to opt in.

`CRDB_DATABASE_URL` (a full libpq connection string) takes precedence over the
split vars below. In both auth modes `sslmode` must be `require`, `verify-ca`,
or `verify-full`; a `CRDB_DATABASE_URL` that omits `sslmode` is rejected at
startup, and the TLS-optional modes (`disable`, `allow`, `prefer`) require
`CRDB_MCP_ALLOW_INSECURE_DB=true`.

| Variable | Purpose | Default |
| --- | --- | --- |
| `CRDB_HOST` | Hostname | required |
| `CRDB_PORT` | Port | `26257` |
| `CRDB_USERNAME` | SQL user | required |
| `CRDB_PWD` | Password (discouraged, see note above) | - |
| `CRDB_SSL_MODE` | `require`, `verify-ca`, or `verify-full`; `disable`, `allow`, `prefer` with `CRDB_MCP_ALLOW_INSECURE_DB=true` | `verify-full` |
| `CRDB_SSL_CA_PATH` | CA cert path (required for `verify-ca` / `verify-full`) | - |
| `CRDB_SSL_CERTFILE` | Client cert path | required |
| `CRDB_SSL_KEYFILE` | Client key path | required |

### Query behavior

| Variable | Purpose | Default |
| --- | --- | --- |
| `CRDB_MCP_QUERY_TIMEOUT` | Per-query timeout (Go duration, e.g. `30s`) | `30s` |
| `CRDB_MCP_MAX_ROWS_COUNT` | Cap on the max LIMIT list-style tools issue to CRDB | `10000` |
| `CRDB_MCP_MAX_CONNS` | Upper bound on the pgxpool connection count (≤ 100) | `10` |
| `CRDB_MCP_TXN_QOS` | Default transaction QoS: `background`, `regular`, or `critical` | `background` |
| `CRDB_MCP_ENABLE_WRITE_QUERIES` | Gate for the write tools (`create_database`, `create_table`, `insert_rows`, `update_rows`, `delete_rows`) | `false` |
| `CRDB_MCP_ALLOW_PASSWORD_AUTH` | Opt-in to password-based auth | `false` |
| `CRDB_MCP_ALLOW_INSECURE_DB` | Opt-in to the sslmode values that can run without TLS: `disable`, `allow`, `prefer`. Cert env vars are not required in these modes. Development only | `false` |

MCP traffic runs at `default_transaction_quality_of_service=background` by
default so it does not contend with latency-sensitive foreground workloads.
Precedence for picking the value:

1. `CRDB_MCP_TXN_QOS` if set (`background`, `regular`, or `critical`).
2. Otherwise, a `default_transaction_quality_of_service=...` query param in
   `CRDB_DATABASE_URL`, if present.
3. Otherwise, `background`.

### Transport

| Variable | Purpose | Default |
| --- | --- | --- |
| `CRDB_MCP_TRANSPORT` | `stdio` or `http` | `stdio` |
| `CRDB_MCP_HTTP_LISTEN_ADDR` | Listen address in HTTP mode, any `host:port`. Omit the host to bind all interfaces. Examples: `:8080`, `0.0.0.0:8443`, `127.0.0.1:9090` | `:8080` |
| `CRDB_MCP_BEARER_TOKEN` | Bearer token clients must send in `Authorization: Bearer <token>`. Required in HTTP mode unless `CRDB_MCP_ALLOW_NO_BEARER=true`; must be at least 16 chars | - |
| `CRDB_MCP_TLS_CERT` | PEM-encoded server cert path. Required for HTTPS unless `CRDB_MCP_ALLOW_INSECURE_HTTP=true` | - |
| `CRDB_MCP_TLS_KEY` | PEM-encoded private key path. Required for HTTPS unless `CRDB_MCP_ALLOW_INSECURE_HTTP=true` | - |
| `CRDB_MCP_ALLOW_INSECURE_HTTP` | Opt-in to cleartext HTTP (for reverse-proxy deployments) | `false` |
| `CRDB_MCP_ALLOW_NO_BEARER` | Opt-in to skip bearer auth (for upstream-auth deployments). Startup warning logged | `false` |

### Logging

| Variable | Purpose | Default |
| --- | --- | --- |
| `CRDB_MCP_LOG_LEVEL` | `debug`, `info`, `warn`, `error` | `info` |
| `CRDB_MCP_LOG_PATH` | Log file path, or `-` for stderr. No rotation; use logrotate or your orchestrator | - |

### Tracing (OpenTelemetry)

Tracing is opt-in: with neither variable below set, no exporter is installed.
When enabled, tool calls and their SQL statements are exported as spans, and
server logs as OTel log records. Query text and errors are redacted before
export so literals and user data never leave the server.

| Variable | Purpose | Default |
| --- | --- | --- |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP gRPC endpoint; the standard `OTEL_*` env vars are honored | - |
| `CRDB_MCP_OTEL_FILE` | Write traces and logs as JSON lines to this file instead (takes precedence) | - |

In stdio mode, add the variable to the `env` block of your
[MCP client config](#setup---mcp-client-config); in HTTP mode, export it in
the server's environment:

```json
{
  "env": {
    "CRDB_DATABASE_URL": "postgresql://...",
    "OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4317"
  }
}
```

## Tools

Grant the connecting SQL role only the privileges the registered tools need.
Avoid admin and write privileges unless write tools are explicitly registered;
see [Security](#security) for a role recipe.

`CRDB_MCP_ENABLE_WRITE_QUERIES` enables all write tools together. For
per-operation control, grant the connecting role only the SQL privileges you
want, for example `SELECT`, `INSERT`, and `UPDATE` but not `DELETE` to allow
writes without deletes.

### Read-only (registered by default)

| Tool | Description |
| --- | --- |
| `list_databases` | List all databases. Optional `limit` (default 100, max 10000) and `offset`. |
| `list_tables` | List tables in a database. Required: `database`. Optional: `limit`, `offset`. |
| `get_table_schema` | Return the `CREATE TABLE` for a table. Required: `database`, `table`. Optional: `schema` (default `public`). |
| `get_cluster` | Cluster identity and version metadata: cluster_id, cluster_name, binary_version, active_version. |
| `list_sql_users` | SQL users in the cluster. Optional: `limit`, `offset`. |
| `list_cluster_nodes` | Cluster nodes with address, liveness, locality. Requires admin or `VIEWCLUSTERMETADATA`. |
| `show_running_queries` | Currently executing statements, ordered by start time descending. Optional: `limit`, `offset`. |
| `select_query` | Execute a SELECT statement. A default LIMIT of 100 is appended when none is supplied; cap is `CRDB_MCP_MAX_ROWS_COUNT`. |
| `explain_query` | Return the EXPLAIN plan for a statement without executing it. `EXPLAIN ANALYZE` (and `EXPLAIN ANALYZE (DEBUG)`) is rejected. Display options (`VERBOSE`, `DISTSQL`, `TYPES`, `OPT`, etc.) pass through. |
| `show_statement` | Execute a SHOW statement such as `SHOW SCHEMAS`, `SHOW INDEXES`, `SHOW REGIONS`, `SHOW JOBS`. Optional: `limit`, `offset`. |

### Write (requires `CRDB_MCP_ENABLE_WRITE_QUERIES=true`)

| Tool | Description |
| --- | --- |
| `create_database` | Create a database. Required: `name`. |
| `create_table` | Execute a single `CREATE TABLE` statement. Required: `statement`. |
| `insert_rows` | Execute a single `INSERT` statement; returns rows affected. Required: `statement`. |
| `update_rows` | Execute a single `UPDATE` statement; a `WHERE` clause is mandatory. Returns rows affected. Required: `statement`. |
| `delete_rows` | Execute a single `DELETE` statement; a `WHERE` clause is mandatory. Returns rows affected. Required: `statement`. |

Any write tool that includes a `RETURNING` clause returns the affected rows instead of just the count.

## Testing

```bash
go test ./...
```

Interactive inspection via [MCP Inspector](https://github.com/modelcontextprotocol/inspector):

```bash
npm install -g @modelcontextprotocol/inspector
mcp-inspector cockroachdb-mcp-server
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Security

Report vulnerabilities via [SECURITY.md](SECURITY.md).

### Security model

The server validates statement shape (a single statement per call, a
mandatory `WHERE` clause on `UPDATE` and `DELETE`) but does not restrict
which databases, schemas, or tables a statement may target. Every tool call
runs with the full privileges of the connecting SQL role: the role, not the
server, is the security boundary.

Connect as a dedicated role scoped to just what the agent needs, never as
`root` or an `admin` member; see
[`CREATE ROLE`](https://www.cockroachlabs.com/docs/stable/create-role),
[`GRANT`](https://www.cockroachlabs.com/docs/stable/grant), and the
[authorization overview](https://www.cockroachlabs.com/docs/stable/security-reference/authorization).
Grant `SELECT` on the tables the agent should see, add DML privileges per
table only when write mode is on, and `GRANT SYSTEM VIEWACTIVITYREDACTED,
VIEWCLUSTERMETADATA` if the cluster introspection tools are needed.

Note that the server cannot tell an operator-intended tool call from one
induced by attacker-controlled content the agent has read (prompt
injection); the role scoping above is the containment for that. MCP tool
annotations are hints to the client, not a server-side control.

## License

[Apache License 2.0](LICENSE).
