package otel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/config"
	"github.com/cockroachdb/cockroachdb-mcp-server/logging"
	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	otelapi "go.opentelemetry.io/otel"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestSetup(t *testing.T) {
	t.Run("no exporter configured returns noop shutdown", func(t *testing.T) {
		ctx := context.Background()
		shutdown, err := Setup(ctx, &config.Config{}, "svc", "v0")
		require.NoError(t, err)
		require.NotNil(t, shutdown)
		require.NoError(t, shutdown(ctx))
	})

	t.Run("file exporter writes spans as JSON", func(t *testing.T) {
		ctx := context.Background()
		path := filepath.Join(t.TempDir(), "otel.jsonl")
		cfg := &config.Config{OTelFile: path}

		shutdown, err := Setup(ctx, cfg, "test-service", "v1.2.3")
		require.NoError(t, err)

		_, span := otelapi.Tracer("test").Start(ctx, "unit-span")
		span.End()

		require.NoError(t, shutdown(ctx))

		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Greater(t, len(raw), 0, "expected exporter to write at least one span")

		var span0 map[string]any
		dec := json.NewDecoder(strings.NewReader(string(raw)))
		require.NoError(t, dec.Decode(&span0))
		require.Equal(t, "unit-span", span0["Name"])
	})

	t.Run("file exporter writes metrics as JSON", func(t *testing.T) {
		ctx := context.Background()
		path := filepath.Join(t.TempDir(), "otel.jsonl")
		cfg := &config.Config{OTelFile: path}

		shutdown, err := Setup(ctx, cfg, "test-service", "v1.2.3")
		require.NoError(t, err)

		hist, err := otelapi.Meter("test").Float64Histogram("unit-histogram")
		require.NoError(t, err)
		hist.Record(ctx, 0.42)

		require.NoError(t, shutdown(ctx))

		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Contains(t, string(raw), "unit-histogram",
			"expected exporter to write the recorded metric")
	})

	t.Run("missing parent dir surfaces a wrapped error", func(t *testing.T) {
		ctx := context.Background()
		cfg := &config.Config{OTelFile: "/nonexistent/dir/otel.jsonl"}
		_, err := Setup(ctx, cfg, "svc", "v0")
		require.Error(t, err)
		require.Contains(t, err.Error(), "open otel file")
	})
}

func TestSafeSpanError(t *testing.T) {
	t.Run("pgx error is reduced to sqlstate", func(t *testing.T) {
		pgErr := &pgconn.PgError{
			Code:    "42P01",
			Message: `relation "secret_users" does not exist`,
			Where:   `SELECT * FROM secret_users WHERE ssn = '123-45-6789'`,
		}
		wrapped := errors.Wrap(pgErr, "exec query")
		safe := SafeSpanError(wrapped)
		require.Contains(t, safe.Error(), "sqlstate 42P01")
		require.NotContains(t, safe.Error(), "secret_users")
		require.NotContains(t, safe.Error(), "123-45-6789")
	})

	t.Run("non-pgx error passes through unchanged", func(t *testing.T) {
		raw := errors.New("connection reset by peer")
		require.Same(t, raw, SafeSpanError(raw))
	})
}

func TestAttachZapBridge(t *testing.T) {
	t.Run("returns a logger that still emits at requested level", func(t *testing.T) {
		base, err := logging.NewLogger(zapcore.InfoLevel, "")
		require.NoError(t, err)
		wrapped := AttachZapBridge(base, "test-service")
		require.NotNil(t, wrapped)
		require.True(t, wrapped.Core().Enabled(zapcore.InfoLevel))
	})

	t.Run("bridge does not panic without provider configured", func(t *testing.T) {
		base, err := logging.NewLogger(zapcore.InfoLevel, "")
		require.NoError(t, err)
		wrapped := AttachZapBridge(base, "test-service")
		require.NotPanics(t, func() {
			wrapped.Info("hello", zap.String("k", "v"))
		})
	})
}
