package config

import "github.com/kelseyhightower/envconfig"

type Config struct {
	Port        string `envconfig:"PORT" default:"8080"`
	DatabaseURL string `envconfig:"DATABASE_URL" required:"true"`
	// SyncToken, when set, is required (X-Sync-Token header) on the data-sync
	// endpoints. Empty leaves them open.
	SyncToken string `envconfig:"SYNC_TOKEN"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
