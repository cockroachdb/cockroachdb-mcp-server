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

// ListTablesParams contains parameters for the list_tables tool.
type ListTablesParams struct {
	Database string `json:"database" jsonschema:"Database whose tables are listed."`
	PaginationParams
}

// TableSchemaParams contains parameters for the get_table_schema tool. Schema
// defaults to "public" when empty.
type TableSchemaParams struct {
	Database string `json:"database" jsonschema:"Database containing the table."`
	Schema   string `json:"schema,omitempty" jsonschema:"Schema name. Defaults to 'public' when omitted."`
	Table    string `json:"table" jsonschema:"Table name."`
}

// ShowRunningQueriesParams contains parameters for the show_running_queries tool.
type ShowRunningQueriesParams struct {
	PaginationParams
}

// ListSQLUsersParams contains parameters for the list_sql_users tool.
type ListSQLUsersParams struct {
	PaginationParams
}

// SelectQueryParams contains parameters for the select_query tool.
type SelectQueryParams struct {
	Query string `json:"query" jsonschema:"A single SELECT statement; non-SELECT statements are rejected. A default LIMIT is appended when none is supplied, capped at CRDB_MCP_MAX_ROWS_COUNT."`
}
