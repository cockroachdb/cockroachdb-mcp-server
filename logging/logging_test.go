package logging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNewLogger(t *testing.T) {
	t.Run("level gating", func(t *testing.T) {
		logger, err := NewLogger(zapcore.WarnLevel, "")
		require.NoError(t, err)
		require.NotNil(t, logger)

		require.True(t, logger.Core().Enabled(zapcore.WarnLevel))
		require.True(t, logger.Core().Enabled(zapcore.ErrorLevel))
		require.False(t, logger.Core().Enabled(zapcore.InfoLevel))
		require.False(t, logger.Core().Enabled(zapcore.DebugLevel))
	})

	t.Run("debug level enables debug", func(t *testing.T) {
		logger, err := NewLogger(zapcore.DebugLevel, "")
		require.NoError(t, err)
		require.True(t, logger.Core().Enabled(zapcore.DebugLevel))
	})

	t.Run("writes JSON records to the given file path", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "mcp.log")
		logger, err := NewLogger(zapcore.InfoLevel, path)
		require.NoError(t, err)
		logger.Info("hello", zap.String("k", "v"))
		require.NoError(t, logger.Sync())

		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		var rec map[string]any
		require.NoError(t, json.Unmarshal(raw, &rec), "file is not JSON: %s", raw)
		require.Equal(t, "info", rec["level"])
		require.Equal(t, "hello", rec["msg"])
		require.Equal(t, "v", rec["k"])
	})

	t.Run("dash routes to stderr", func(t *testing.T) {
		logger, err := NewLogger(zapcore.InfoLevel, "-")
		require.NoError(t, err)
		require.NotNil(t, logger)
	})

	t.Run("unwritable path is rejected", func(t *testing.T) {
		_, err := NewLogger(zapcore.InfoLevel, "/nonexistent/dir/mcp.log")
		require.Error(t, err)
	})
}
