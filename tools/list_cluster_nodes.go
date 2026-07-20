package tools

import (
	"context"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const listClusterNodesQuery = `
SELECT g.node_id, g.address, g.sql_address, g.build_tag, g.started_at, g.is_live, k.locality
FROM crdb_internal.gossip_nodes g
LEFT JOIN crdb_internal.kv_node_status k USING (node_id)
ORDER BY g.node_id`

// listClusterNodes handles the list_cluster_nodes tool.
func (h *ToolHandlers) listClusterNodes(
	ctx context.Context, _ *mcp.CallToolRequest, _ struct{},
) (*mcp.CallToolResult, any, error) {
	res, err := h.dm.Query(ctx, listClusterNodesQuery)
	if err != nil {
		if db.IsInsufficientPrivilege(err) {
			return nil, nil, errors.Wrap(err,
				"list cluster nodes: access denied; requires admin or VIEWCLUSTERMETADATA, or "+
					restrictedInternalsMsg)
		}
		return nil, nil, errors.Wrap(err, "list cluster nodes")
	}
	result, err := queryResultToMCP(res)
	return result, nil, err
}
