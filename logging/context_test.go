package logging

import (
	"bytes"
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.opentelemetry.io/otel"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// recordExporter captures the log records the otelzap bridge emits.
type recordExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *recordExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.records = append(e.records, records...)
	return nil
}

func (e *recordExporter) Shutdown(context.Context) error   { return nil }
func (e *recordExporter) ForceFlush(context.Context) error { return nil }

// withTracer installs a real tracer provider for the duration of the test.
func withTracer(t *testing.T) {
	t.Helper()
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider())
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
}

// withBridge points the global logger at the otelzap bridge feeding exp.
func withBridge(t *testing.T) *recordExporter {
	t.Helper()
	exp := &recordExporter{}
	lp := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exp)))
	t.Cleanup(func() { _ = lp.Shutdown(context.Background()) })
	t.Cleanup(zap.ReplaceGlobals(zap.New(otelzap.NewCore("test", otelzap.WithLoggerProvider(lp)))))
	return exp
}

func TestL(t *testing.T) {
	t.Run("no span leaves the record without trace context", func(t *testing.T) {
		exp := withBridge(t)
		L(context.Background()).Info("hello")

		require.Len(t, exp.records, 1)
		require.False(t, exp.records[0].TraceID().IsValid())
	})

	t.Run("valid span stamps native trace and span IDs on the record", func(t *testing.T) {
		withTracer(t)
		exp := withBridge(t)
		ctx, span := otel.Tracer("t").Start(context.Background(), "op")
		L(ctx).Info("hello")
		span.End()

		require.Len(t, exp.records, 1)
		require.Equal(t, span.SpanContext().TraceID(), exp.records[0].TraceID())
		require.Equal(t, span.SpanContext().SpanID(), exp.records[0].SpanID())
	})

	t.Run("ctx never reaches the serialized log line", func(t *testing.T) {
		withTracer(t)
		var buf bytes.Buffer
		enc := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
		t.Cleanup(zap.ReplaceGlobals(zap.New(zapcore.NewCore(enc, zapcore.AddSync(&buf), zapcore.DebugLevel))))

		ctx, span := otel.Tracer("t").Start(context.Background(), "op")
		L(ctx).Info("hello", zap.String("k", "v"))
		span.End()

		out := buf.String()
		require.Contains(t, out, `"k":"v"`, "real fields still serialize")
		require.NotContains(t, out, "otel.ctx", "smuggled ctx field must not serialize")
	})
}
