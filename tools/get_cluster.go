package tools

import (
	"context"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Run independently: version() always works, crdb_internal identity builtins
// may be gated (SQLSTATE 42501) on v25.4+.
const (
	getClusterBinaryVersionQuery = `SELECT version() AS binary_version`
	getClusterIdentityQuery      = `
SELECT crdb_internal.cluster_id()::STRING AS cluster_id,
       crdb_internal.cluster_name()       AS cluster_name,
       crdb_internal.active_version()     AS active_version`
)

const restrictedInternalsReason = restrictedInternalsMsg + "; no privilege grants access"

// getCluster handles the get_cluster tool. Fields the connection cannot
// access stay null with the reason keyed under "unavailable".
func (h *ToolHandlers) getCluster(
	ctx context.Context, _ *mcp.CallToolRequest, _ struct{},
) (*mcp.CallToolResult, any, error) {
	binaryRes, err := h.dm.Query(ctx, getClusterBinaryVersionQuery)
	if err != nil {
		return nil, nil, errors.Wrap(err, "get cluster")
	}
	payload := map[string]any{
		"cluster_id":      nil,
		"cluster_name":    nil,
		"binary_version":  firstValue(binaryRes),
		"active_version":  nil,
	}
	unavailable := map[string]string{}

	identityRes, err := h.dm.Query(ctx, getClusterIdentityQuery)
	switch {
	case err == nil:
		if identityRes != nil && len(identityRes.Rows) > 0 && len(identityRes.Rows[0]) > 2 {
			payload["cluster_id"] = identityRes.Rows[0][0]
			payload["cluster_name"] = identityRes.Rows[0][1]
			payload["active_version"] = identityRes.Rows[0][2]
		}
	case db.IsInsufficientPrivilege(err):
		unavailable["cluster_id"] = restrictedInternalsReason
		unavailable["cluster_name"] = restrictedInternalsReason
		unavailable["active_version"] = restrictedInternalsReason
	default:
		return nil, nil, errors.Wrap(err, "get cluster")
	}

	if len(unavailable) > 0 {
		payload["unavailable"] = unavailable
	}
	return writeOK(payload)
}

// firstValue returns the first column of the first row, or nil when the
// result is empty.
func firstValue(res *db.QueryResult) any {
	if res == nil || len(res.Rows) == 0 || len(res.Rows[0]) == 0 {
		return nil
	}
	return res.Rows[0][0]
}
