package tools

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// getClusterQuery returns a single row.
const getClusterQuery = `
SELECT crdb_internal.cluster_id()     AS cluster_id,
       crdb_internal.cluster_name()   AS cluster_name,
       version()                      AS binary_version,
       crdb_internal.active_version() AS active_version`

// getCluster handles the get_cluster tool.
func (h *ToolHandlers) getCluster(
	ctx context.Context, _ *mcp.CallToolRequest, _ struct{},
) (*mcp.CallToolResult, any, error) {
	res, err := h.dm.Query(ctx, getClusterQuery)
	if err != nil {
		return nil, nil, errors.Wrap(err, "get cluster")
	}
	result, err := queryResultToMCP(res)
	return result, nil, err
}
