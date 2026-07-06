package middleware

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

// ToolCallMetrics records a duration histogram for every tools/call, keyed by
// tool name and outcome. Unlike per-call logs, histogram accuracy survives
// log sampling, so operators can derive p50/p95/p99, throughput, and error
// rate even with tool-call logs dropped. No-op until otel.Setup installs an
// exporter.
func ToolCallMetrics(next mcp.MethodHandler) mcp.MethodHandler {
	hist, err := otel.Meter(tracerScope).Float64Histogram(
		"mcp.tool.call.duration",
		metric.WithDescription("Duration of MCP tool calls."),
		metric.WithUnit("s"),
	)
	if err != nil {
		zap.L().Warn("tool-call duration histogram unavailable", zap.Error(err))
		return next
	}
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		if method != methodCallTool {
			return next(ctx, method, req)
		}
		start := time.Now()
		result, err := next(ctx, method, req)
		outcome := "success"
		switch {
		case err != nil:
			outcome = "error"
		case isErrorResult(result):
			outcome = "tool_error"
		}
		hist.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(
				attribute.String("mcp.tool.name", toolName(req)),
				attribute.String("mcp.tool.call.outcome", outcome),
			))
		return result, err
	}
}
