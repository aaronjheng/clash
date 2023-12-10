package hub

import (
	"github.com/Dreamacro/clash/internal/config"
	"github.com/Dreamacro/clash/internal/hub/executor"
)

type Option func(*config.Config)

func WithExternalController(externalController string) Option {
	return func(cfg *config.Config) {
		cfg.General.ExternalController = externalController
	}
}

func WithSecret(secret string) Option {
	return func(cfg *config.Config) {
		cfg.General.Secret = secret
	}
}

// Parse call at the beginning of clash
func Parse(options ...Option) error {
	cfg, err := executor.Parse()
	if err != nil {
		return err
	}

	for _, option := range options {
		option(cfg)
	}

	executor.ApplyConfig(cfg, true)
	return nil
}
