package tools

// PaginationParams contains optional pagination parameters for list operations.
type PaginationParams struct {
	Limit  *int64 `json:"limit,omitempty" jsonschema:"Max rows to return. Defaults to 100, capped at CRDB_MCP_MAX_ROWS_COUNT (default 10000)."`
	Offset *int64 `json:"offset,omitempty" jsonschema:"Rows to skip before returning results. Must be zero or positive. Defaults to 0."`
}

// ListDatabasesParams contains parameters for the list_databases tool.
type ListDatabasesParams struct {
	PaginationParams
}
