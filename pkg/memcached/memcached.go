package memcached

import (
	"context"
	"net/http"
	"time"
)

type IMemcached interface {
	Get(ctx context.Context, key string) (CacheItem, error)
	Set(ctx context.Context, key string, item CacheItem, ttl time.Duration) error
	Ping() error
}

type CacheItem struct {
	Body     []byte
	Status   int
	Header   http.Header
	StoredAt int64

	// MaxAge stores the expiration of the cache.
	// Equivalent to TTL
	MaxAge int64

	// Age represents duration of the content stored in the cache in seconds.
	Age int64
}
