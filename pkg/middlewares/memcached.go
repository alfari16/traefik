package middlewares

import (
	"context"
	"time"
)

type IMemcachedHandler[K any] interface {
	Get(ctx context.Context, key string, dst *K) error
	Set(ctx context.Context, key string, item K, ttl time.Duration) error
	Ping() error
}
