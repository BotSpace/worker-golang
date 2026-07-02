package logger

import "go.uber.org/zap"

func New() (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.DisableStacktrace = true
	return cfg.Build()
}
