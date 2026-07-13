package logging

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// L returns the global zap logger carrying ctx so the otelzap bridge stamps the
// span's native trace context onto exported log records. No span, no change.
func L(ctx context.Context) *zap.Logger {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return zap.L()
	}
	return zap.L().With(ctxField(ctx))
}

// ctxField smuggles ctx to the otelzap bridge; SkipType keeps it out of the log line.
func ctxField(ctx context.Context) zap.Field {
	return zapcore.Field{Key: "otel.ctx", Type: zapcore.SkipType, Interface: ctx}
}
