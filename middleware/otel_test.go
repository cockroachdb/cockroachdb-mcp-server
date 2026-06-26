package middleware

import (
	"context"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestToolCallSpan(t *testing.T) {
	const toolName = "fake_tool"

	withRecorder := func(t *testing.T) *tracetest.SpanRecorder {
		t.Helper()
		rec := tracetest.NewSpanRecorder()
		tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
		prev := otel.GetTracerProvider()
		otel.SetTracerProvider(tp)
		t.Cleanup(func() { otel.SetTracerProvider(prev) })
		return rec
	}

	callToolReq := func() mcp.Request {
		return &mcp.ServerRequest[*mcp.CallToolParamsRaw]{Params: &mcp.CallToolParamsRaw{Name: toolName}}
	}

	t.Run("successful call records a span named after the tool", func(t *testing.T) {
		rec := withRecorder(t)
		next := func(context.Context, string, mcp.Request) (mcp.Result, error) {
			return &mcp.CallToolResult{}, nil
		}
		_, err := ToolCallSpan(next)(context.Background(), methodCallTool, callToolReq())
		require.NoError(t, err)

		spans := rec.Ended()
		require.Len(t, spans, 1)
		require.Equal(t, toolName, spans[0].Name())
		require.Equal(t, "Unset", spans[0].Status().Code.String())
		require.Equal(t, "server", spans[0].SpanKind().String())
	})

	t.Run("transport error sets Error status and records the error", func(t *testing.T) {
		rec := withRecorder(t)
		boom := errors.New("boom")
		next := func(context.Context, string, mcp.Request) (mcp.Result, error) {
			return nil, boom
		}
		_, err := ToolCallSpan(next)(context.Background(), methodCallTool, callToolReq())
		require.ErrorIs(t, err, boom)

		spans := rec.Ended()
		require.Len(t, spans, 1)
		require.Equal(t, "Error", spans[0].Status().Code.String())
		require.NotEmpty(t, spans[0].Events(), "RecordError should have added an event")
	})

	t.Run("empty tool name falls back to unknown_tool", func(t *testing.T) {
		rec := withRecorder(t)
		next := func(context.Context, string, mcp.Request) (mcp.Result, error) {
			return &mcp.CallToolResult{}, nil
		}
		req := &mcp.ServerRequest[*mcp.CallToolParamsRaw]{Params: &mcp.CallToolParamsRaw{}}
		_, err := ToolCallSpan(next)(context.Background(), methodCallTool, req)
		require.NoError(t, err)

		spans := rec.Ended()
		require.Len(t, spans, 1)
		require.Equal(t, "unknown_tool", spans[0].Name())
	})

	t.Run("database error is sanitized before recording on the span", func(t *testing.T) {
		rec := withRecorder(t)
		pgErr := &pgconn.PgError{
			Code:    "42P01",
			Message: `relation "secret_users" does not exist`,
			Where:   `SELECT * FROM secret_users WHERE ssn = '123-45-6789'`,
		}
		wrapped := errors.Wrap(pgErr, "list databases")
		next := func(context.Context, string, mcp.Request) (mcp.Result, error) {
			return nil, wrapped
		}
		_, err := ToolCallSpan(next)(context.Background(), methodCallTool, callToolReq())
		require.ErrorIs(t, err, wrapped)

		spans := rec.Ended()
		require.Len(t, spans, 1)
		require.Equal(t, "Error", spans[0].Status().Code.String())
		require.Contains(t, spans[0].Status().Description, "sqlstate 42P01")
		require.NotContains(t, spans[0].Status().Description, "secret_users")
		events := spans[0].Events()
		require.NotEmpty(t, events)
		for _, ev := range events {
			for _, attr := range ev.Attributes {
				require.NotContains(t, attr.Value.AsString(), "secret_users")
				require.NotContains(t, attr.Value.AsString(), "123-45-6789")
			}
		}
	})

	t.Run("CallToolResult.IsError sets Error status without an event", func(t *testing.T) {
		rec := withRecorder(t)
		next := func(context.Context, string, mcp.Request) (mcp.Result, error) {
			return &mcp.CallToolResult{IsError: true}, nil
		}
		_, err := ToolCallSpan(next)(context.Background(), methodCallTool, callToolReq())
		require.NoError(t, err)

		spans := rec.Ended()
		require.Len(t, spans, 1)
		require.Equal(t, "Error", spans[0].Status().Code.String())
	})

	t.Run("non-tool method bypasses span creation", func(t *testing.T) {
		rec := withRecorder(t)
		next := func(context.Context, string, mcp.Request) (mcp.Result, error) {
			return nil, nil
		}
		_, err := ToolCallSpan(next)(context.Background(), "notifications/initialized",
			&mcp.ServerRequest[*mcp.CallToolParamsRaw]{Params: &mcp.CallToolParamsRaw{}})
		require.NoError(t, err)
		require.Empty(t, rec.Ended())
	})
}
