package dev

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ListDatabases is a placeholder tool that will list databases
func ListDatabases(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: "TODO: Database connection not yet implemented",
			},
		},
	}, nil, nil
}
