package tools

import (
	"encoding/json"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultListDatabasesLimit int64 = 100
	defaultListTablesLimit    int64 = 100
)

// MCPQueryResult is the response shape returned by read tools.
type MCPQueryResult struct {
	Rows []json.RawMessage `json:"rows"`
}

// queryResultToMCP renders a db.QueryResult as an MCP tool response.
func queryResultToMCP(qr *db.QueryResult) (*mcp.CallToolResult, error) {
	rows := make([]json.RawMessage, 0, len(qr.Rows))
	for _, row := range qr.Rows {
		m := make(map[string]any, len(qr.Columns))
		for i, col := range qr.Columns {
			m[col] = row[i]
		}
		raw, err := json.Marshal(m)
		if err != nil {
			return nil, errors.Wrap(err, "marshal row")
		}
		rows = append(rows, raw)
	}
	data, err := json.Marshal(&MCPQueryResult{Rows: rows})
	if err != nil {
		return nil, errors.Wrap(err, "marshal query result")
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, nil
}

// applyLimitOffset appends LIMIT and OFFSET clauses via db.SafeFormat.
// Non-positive limits and negative offsets are rejected, and limits are capped
// at the server-configured MaxRowsCount.
func (h *ToolHandlers) applyLimitOffset(query string, limit, offset *int64, defaultLimit int64) (string, error) {
	if limit != nil && *limit <= 0 {
		return "", errors.New("LIMIT must be a positive integer")
	}
	if offset != nil && *offset < 0 {
		return "", errors.New("OFFSET must be zero or positive")
	}
	limitVal := defaultLimit
	if limit != nil {
		limitVal = min(*limit, h.cfg.MaxRowsCount)
	}
	var offsetVal int64
	if offset != nil && *offset > 0 {
		offsetVal = *offset
	}
	if offsetVal > 0 {
		return db.SafeFormat("%1 LIMIT %2 OFFSET %3", db.SQL(query), limitVal, offsetVal)
	}
	return db.SafeFormat("%1 LIMIT %2", db.SQL(query), limitVal)
}
