package middleware

import (
	"context"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestToolCallMetrics(t *testing.T) {
	const tool = "fake_tool"

	withReader := func(t *testing.T) *sdkmetric.ManualReader {
		t.Helper()
		reader := sdkmetric.NewManualReader()
		mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		prev := otel.GetMeterProvider()
		otel.SetMeterProvider(mp)
		t.Cleanup(func() { otel.SetMeterProvider(prev) })
		return reader
	}

	callToolReq := func() mcp.Request {
		return &mcp.ServerRequest[*mcp.CallToolParamsRaw]{Params: &mcp.CallToolParamsRaw{Name: tool}}
	}

	dataPoints := func(t *testing.T, reader *sdkmetric.ManualReader) []metricdata.HistogramDataPoint[float64] {
		t.Helper()
		var rm metricdata.ResourceMetrics
		require.NoError(t, reader.Collect(context.Background(), &rm))
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name != "mcp.tool.call.duration" {
					continue
				}
				h, ok := m.Data.(metricdata.Histogram[float64])
				require.True(t, ok, "expected float64 histogram data")
				return h.DataPoints
			}
		}
		return nil
	}

	attrValue := func(t *testing.T, dp metricdata.HistogramDataPoint[float64], key string) string {
		t.Helper()
		v, ok := dp.Attributes.Value(attribute.Key(key))
		require.True(t, ok, "missing attribute %s", key)
		return v.AsString()
	}

	run := func(t *testing.T, next mcp.MethodHandler) *sdkmetric.ManualReader {
		t.Helper()
		reader := withReader(t)
		_, _ = ToolCallMetrics(next)(context.Background(), methodCallTool, callToolReq())
		return reader
	}

	t.Run("successful call records duration keyed by tool and outcome", func(t *testing.T) {
		reader := run(t, func(context.Context, string, mcp.Request) (mcp.Result, error) {
			return &mcp.CallToolResult{}, nil
		})
		dps := dataPoints(t, reader)
		require.Len(t, dps, 1)
		require.Equal(t, uint64(1), dps[0].Count)
		require.GreaterOrEqual(t, dps[0].Sum, 0.0)
		require.Equal(t, tool, attrValue(t, dps[0], "mcp.tool.name"))
		require.Equal(t, "success", attrValue(t, dps[0], "mcp.tool.call.outcome"))
	})

	t.Run("transport error records outcome error", func(t *testing.T) {
		reader := run(t, func(context.Context, string, mcp.Request) (mcp.Result, error) {
			return nil, errors.New("boom")
		})
		dps := dataPoints(t, reader)
		require.Len(t, dps, 1)
		require.Equal(t, "error", attrValue(t, dps[0], "mcp.tool.call.outcome"))
	})

	t.Run("CallToolResult.IsError records outcome tool_error", func(t *testing.T) {
		reader := run(t, func(context.Context, string, mcp.Request) (mcp.Result, error) {
			return &mcp.CallToolResult{IsError: true}, nil
		})
		dps := dataPoints(t, reader)
		require.Len(t, dps, 1)
		require.Equal(t, "tool_error", attrValue(t, dps[0], "mcp.tool.call.outcome"))
	})

	t.Run("non-tool method records nothing", func(t *testing.T) {
		reader := withReader(t)
		next := func(context.Context, string, mcp.Request) (mcp.Result, error) {
			return nil, nil
		}
		_, err := ToolCallMetrics(next)(context.Background(), "notifications/initialized",
			&mcp.ServerRequest[*mcp.CallToolParamsRaw]{Params: &mcp.CallToolParamsRaw{}})
		require.NoError(t, err)
		require.Empty(t, dataPoints(t, reader))
	})
}
