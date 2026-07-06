package middleware

import (
	"context"

	mcpotel "github.com/cockroachdb/cockroachdb-mcp-server/otel"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// tracerScope names the instrumentation scope for spans and metrics created
// by this package, per the OTel convention of using the Go import path.
const tracerScope = "github.com/cockroachdb/cockroachdb-mcp-server/middleware"

// toolName extracts the tool name from a tools/call request, falling back to
// a sentinel so unexpected request shapes stay identifiable in trace UIs.
func toolName(req mcp.Request) string {
	if p, ok := req.GetParams().(*mcp.CallToolParamsRaw); ok && p.Name != "" {
		return p.Name
	}
	return "unknown_tool"
}

// ToolCallSpan wraps every tools/call in an OTel span named after the tool.
// No-op until otel.Setup installs an exporter.
func ToolCallSpan(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		if method != methodCallTool {
			return next(ctx, method, req)
		}
		ctx, span := otel.Tracer(tracerScope).Start(ctx, toolName(req),
			trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()
		result, err := next(ctx, method, req)
		switch {
		case err != nil:
			safe := mcpotel.SafeSpanError(err)
			span.RecordError(safe)
			span.SetStatus(codes.Error, safe.Error())
		case isErrorResult(result):
			span.SetStatus(codes.Error, "tool returned error result")
		}
		return result, err
	}
}
