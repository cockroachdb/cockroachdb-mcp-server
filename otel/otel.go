// Package otel wires OpenTelemetry traces and logs for the MCP server.
//
// OTel is opt-in: if neither CRDB_MCP_OTEL_FILE nor OTEL_EXPORTER_OTLP_ENDPOINT
// is set, Setup installs no exporters and the global providers stay no-op.
package otel

import (
	"context"
	"os"

	"github.com/cockroachdb/cockroachdb-mcp-server/config"
	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/contrib/bridges/otelzap"
	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// SafeSpanError strips pgx error Message/Detail/Where fields, which can echo
// the offending SQL and user data into span events and status descriptions,
// undermining the query-text redaction applied to span attributes.
func SafeSpanError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return errors.Newf("cockroachdb error: sqlstate %s", pgErr.Code)
	}
	return err
}

// ShutdownFunc flushes and closes any registered exporters.
type ShutdownFunc func(context.Context) error

// Setup installs trace and log providers based on cfg:
//   - cfg.OTelFile set: stdout exporters write JSON to that file (good for
//     local dev or file-based shippers like Filebeat, Vector).
//   - cfg.OTLPEndpoint set: OTLP gRPC exporters honor the standard OTEL_*
//     env vars (Datadog Agent, OTel Collector, Tempo, Honeycomb, ELK).
//   - neither set: returns a no-op shutdown and the global providers stay
//     no-op.
//
// File mode takes precedence when both are configured.
func Setup(
	ctx context.Context, cfg *config.Config, serviceName, serviceVersion string,
) (ShutdownFunc, error) {
	if !cfg.OTelEnabled() {
		return noopShutdown, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
		resource.WithFromEnv(),
		resource.WithHost(),
		resource.WithProcessRuntimeName(),
	)
	if err != nil {
		return nil, errors.Wrap(err, "build otel resource")
	}

	traceExp, logExp, fileCloser, err := newExporters(ctx, cfg.OTelFile)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)
	otelapi.SetTracerProvider(tp)

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
		sdklog.WithResource(res),
	)
	global.SetLoggerProvider(lp)

	return func(ctx context.Context) error {
		tperr := tp.Shutdown(ctx)
		lperr := lp.Shutdown(ctx)
		var fcerr error
		if fileCloser != nil {
			fcerr = fileCloser.Close()
		}
		return errors.Join(tperr, lperr, fcerr)
	}, nil
}

// AttachZapBridge wraps logger so each record is also exported as an OTel log
// record via the global LoggerProvider. Call only after Setup returned a
// non-nil shutdown (i.e. an exporter is actually configured).
func AttachZapBridge(logger *zap.Logger, serviceName string) *zap.Logger {
	bridge := otelzap.NewCore(serviceName,
		otelzap.WithLoggerProvider(global.GetLoggerProvider()))
	return logger.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
		return zapcore.NewTee(c, bridge)
	}))
}

func newExporters(
	ctx context.Context, filePath string,
) (sdktrace.SpanExporter, sdklog.Exporter, *os.File, error) {
	if filePath != "" {
		f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, nil, nil, errors.Wrapf(err, "open otel file %q", filePath)
		}
		te, err := stdouttrace.New(stdouttrace.WithWriter(f))
		if err != nil {
			_ = f.Close()
			return nil, nil, nil, errors.Wrap(err, "build stdout trace exporter")
		}
		le, err := stdoutlog.New(stdoutlog.WithWriter(f))
		if err != nil {
			_ = te.Shutdown(ctx)
			_ = f.Close()
			return nil, nil, nil, errors.Wrap(err, "build stdout log exporter")
		}
		return te, le, f, nil
	}
	te, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "build otlp trace exporter")
	}
	le, err := otlploggrpc.New(ctx)
	if err != nil {
		_ = te.Shutdown(ctx)
		return nil, nil, nil, errors.Wrap(err, "build otlp log exporter")
	}
	return te, le, nil, nil
}

func noopShutdown(context.Context) error { return nil }
