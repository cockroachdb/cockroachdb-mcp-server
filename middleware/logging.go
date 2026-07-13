// Package middleware holds cross-cutting wrappers for the MCP server's
// receiving method handler (logging, tracing, metrics, etc.). Register one
// via (*mcp.Server).AddReceivingMiddleware.
package middleware

import (
	"context"
	"time"

	"github.com/cockroachdb/cockroachdb-mcp-server/logging"
	mcpotel "github.com/cockroachdb/cockroachdb-mcp-server/otel"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

const methodCallTool = "tools/call"

// ToolCallLogger emits a structured log record for every tools/call with the
// tool name, latency, and result status, via logging.L(ctx) so the active
// span's trace context is attached for log-trace correlation.
func ToolCallLogger(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		if method != methodCallTool {
			return next(ctx, method, req)
		}
		var name string
		if p, ok := req.GetParams().(*mcp.CallToolParamsRaw); ok {
			name = p.Name
		}
		start := time.Now()
		result, err := next(ctx, method, req)
		fields := []zap.Field{
			zap.String("tool", name),
			zap.Int64("duration_ms", time.Since(start).Milliseconds()),
		}
		switch {
		case err != nil:
			// Redact pgx error detail before it reaches the log sink and its
			// OTLP export, matching the span path (see otel.SafeSpanError).
			logging.L(ctx).Error("tool call failed", append(fields, zap.Error(mcpotel.SafeSpanError(err)))...)
		case isErrorResult(result):
			logging.L(ctx).Warn("tool call returned error result", fields...)
		default:
			logging.L(ctx).Info("tool call", fields...)
		}
		return result, err
	}
}

func isErrorResult(r mcp.Result) bool {
	ct, ok := r.(*mcp.CallToolResult)
	return ok && ct.IsError
}
