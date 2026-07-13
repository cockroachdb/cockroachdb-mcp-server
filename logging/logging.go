// Package logging wires the structured logger used by the MCP server.
// Log-trace correlation is opt-in: only records emitted via L(ctx) carry the
// active span's trace context.
package logging

import (
	"github.com/cockroachdb/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger returns a JSON zap logger. Empty path or "-" writes to stderr
// (stdio transport owns stdout); any other value is opened append-only.
func NewLogger(level zapcore.Level, path string) (*zap.Logger, error) {
	encoder := zap.NewProductionEncoderConfig()
	encoder.EncodeTime = zapcore.ISO8601TimeEncoder
	sink := "stderr"
	if path != "" && path != "-" {
		sink = path
	}
	cfg := zap.Config{
		Level:             zap.NewAtomicLevelAt(level),
		Development:       false,
		DisableStacktrace: true,
		Encoding:          "json",
		EncoderConfig:     encoder,
		OutputPaths:       []string{sink},
		ErrorOutputPaths:  []string{"stderr"},
	}
	logger, err := cfg.Build()
	if err != nil {
		return nil, errors.Wrap(err, "build zap logger")
	}
	return logger, nil
}
