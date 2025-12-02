# CockroachDB MCP Server

A Model Context Protocol (MCP) server, that provides tools for interacting with CockroachDB.

## Project Structure

```
cockroachdb-mcp-server/
├── cmd/
│   ├── crdb-mcp-dev/        # Dev server entrypoint
├── internal/
│   ├── auth/                # Authentication (OAuth/JWT/API key)
│   ├── config/              # Configuration loader
│   ├── db/                  # DB client, pooling, guardrails
│   ├── log/                 # Logging and audit
│   ├── otel/                # OpenTelemetry metrics/tracing
│   ├── rate/                # Rate limiting
│   └── tools/               # MCP tool handlers
│       ├── dev/             # Dev server tools
│       └── shared/          # Shared schemas/helpers
└── README.md
```

## Getting Started

### Prerequisites

- Go 1.24 or later
- CockroachDB cluster (local or cloud)

### Installation

```bash
# Clone the repository
git clone https://github.com/cockroachdb/cockroachdb-mcp-server.git
cd cockroachdb-mcp-server

# Build the server
go build -o bin/crdb-mcp-dev ./cmd/crdb-mcp-dev
```

### Quick Start

```bash
# Set database connection
export CRDB_DATABASE_URL="postgresql://root@localhost:26257/defaultdb?sslmode=disable"

# Run in STDIO mode (default)
./bin/crdb-mcp-dev
```

### Testing with MCP Inspector

The [MCP Inspector](https://github.com/modelcontextprotocol/inspector) is a developer tool for testing and debugging MCP servers.

```bash
# Install MCP Inspector
npm install -g @modelcontextprotocol/inspector

# Run the server with MCP Inspector
export CRDB_DATABASE_URL="postgresql://root@localhost:26257/defaultdb?sslmode=disable"
mcp-inspector ./bin/crdb-mcp-dev
```

This will:
1. Start the MCP server
2. Launch a web interface at `http://localhost:5173`
3. Allow you to interactively test tools

## License

[License TBD]

## Contributing

[Contributing guidelines TBD]