package middleware

import (
	"context"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestToolCallLogger(t *testing.T) {
	const toolName = "fake_tool"

	withObservedLogger := func(t *testing.T) *observer.ObservedLogs {
		t.Helper()
		core, recorded := observer.New(zapcore.DebugLevel)
		restore := zap.ReplaceGlobals(zap.New(core))
		t.Cleanup(restore)
		return recorded
	}

	callToolReq := func() mcp.Request {
		return &mcp.ServerRequest[*mcp.CallToolParamsRaw]{Params: &mcp.CallToolParamsRaw{Name: toolName}}
	}

	t.Run("successful call emits info record", func(t *testing.T) {
		recorded := withObservedLogger(t)
		next := func(context.Context, string, mcp.Request) (mcp.Result, error) {
			return &mcp.CallToolResult{}, nil
		}
		_, err := ToolCallLogger(next)(context.Background(), methodCallTool, callToolReq())
		require.NoError(t, err)

		entries := recorded.AllUntimed()
		require.Len(t, entries, 1)
		require.Equal(t, zapcore.InfoLevel, entries[0].Level)
		require.Equal(t, "tool call", entries[0].Message)
		ctx := entries[0].ContextMap()
		require.Equal(t, toolName, ctx["tool"])
		require.Contains(t, ctx, "duration_ms")
	})

	t.Run("transport error emits error record and propagates", func(t *testing.T) {
		recorded := withObservedLogger(t)
		boom := errors.New("boom")
		next := func(context.Context, string, mcp.Request) (mcp.Result, error) {
			return nil, boom
		}
		_, err := ToolCallLogger(next)(context.Background(), methodCallTool, callToolReq())
		require.ErrorIs(t, err, boom)

		entries := recorded.AllUntimed()
		require.Len(t, entries, 1)
		require.Equal(t, zapcore.ErrorLevel, entries[0].Level)
		require.Equal(t, "tool call failed", entries[0].Message)
		require.Equal(t, "boom", entries[0].ContextMap()["error"])
	})

	t.Run("pgx error detail is redacted in the log record", func(t *testing.T) {
		recorded := withObservedLogger(t)
		pgErr := &pgconn.PgError{Code: "42601", Message: `syntax error near "secret_value"`}
		next := func(context.Context, string, mcp.Request) (mcp.Result, error) {
			return nil, errors.Wrap(pgErr, "select query")
		}
		_, err := ToolCallLogger(next)(context.Background(), methodCallTool, callToolReq())
		require.Error(t, err)

		entries := recorded.AllUntimed()
		require.Len(t, entries, 1)
		logged, _ := entries[0].ContextMap()["error"].(string)
		require.Contains(t, logged, "42601")
		require.NotContains(t, logged, "secret_value")
	})

	t.Run("CallToolResult.IsError emits warn record", func(t *testing.T) {
		recorded := withObservedLogger(t)
		next := func(context.Context, string, mcp.Request) (mcp.Result, error) {
			return &mcp.CallToolResult{IsError: true}, nil
		}
		_, err := ToolCallLogger(next)(context.Background(), methodCallTool, callToolReq())
		require.NoError(t, err)

		entries := recorded.AllUntimed()
		require.Len(t, entries, 1)
		require.Equal(t, zapcore.WarnLevel, entries[0].Level)
		require.Equal(t, "tool call returned error result", entries[0].Message)
	})
}
