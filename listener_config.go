package pgxlisten

import (
	"context"
	"time"
)

type listenerConfig struct {
	context       context.Context
	logger        logger
	retryInterval time.Duration
}

func newListenerConfig(opts ...listenerOption) listenerConfig {
	c := listenerConfig{
		context:       context.Background(),
		logger:        noopLogger{},
		retryInterval: time.Minute,
	}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

type listenerOption func(*listenerConfig)

func WithContext(ctx context.Context) listenerOption {
	return func(p *listenerConfig) {
		p.context = ctx
	}
}

func WithLogger(l logger) listenerOption {
	return func(p *listenerConfig) {
		p.logger = l
	}
}

func WithRetryInterval(interval time.Duration) listenerOption {
	return func(p *listenerConfig) {
		p.retryInterval = interval
	}
}
