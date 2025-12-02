package config

import (
	"fmt"
	"os"
)

// Config holds the server configuration
type Config struct {
	// Database connection string
	DatabaseURL string
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL: os.Getenv("CRDB_DATABASE_URL"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("CRDB_DATABASE_URL is required")
	}

	return cfg, nil
}
