package memcached

import (
	"bytes"
	"context"
	"encoding/gob"
	"github.com/bradfitz/gomemcache/memcache"
	"go.skia.org/infra/go/reconnectingmemcached"
	"time"
)

type handler[K any] struct {
	client reconnectingmemcached.Client
}

func NewMemcachedHandler[K any](client *Client) *handler[K] {
	if client == nil {
		return nil
	}
	return &handler[K]{
		client: client.client,
	}
}

func (c *handler[K]) Get(ctx context.Context, key string, dst *K) error {
	if c == nil {
		return ErrMemcachedNotinitialized
	}

	items, ok := c.client.GetMulti([]string{key})
	if !ok {
		return ErrUnknown
	}

	item, ok := items[key]
	if !ok {
		return ErrKeyNotFound{key: key}
	}

	buf := bytes.NewReader(item.Value)

	if err := gob.NewDecoder(buf).Decode(&dst); err != nil {
		return err
	}

	return nil
}

func (c *handler[K]) Set(ctx context.Context, key string, item K, ttl time.Duration) error {
	if c == nil {
		return ErrMemcachedNotinitialized
	}

	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(item); err != nil {
		return err
	}

	ok := c.client.Set(&memcache.Item{
		Key:        key,
		Value:      buf.Bytes(),
		Expiration: int32(ttl.Seconds()),
	})
	if !ok {
		return ErrUnknown
	}

	return nil
}

func (c *handler[K]) Ping() error {
	if c == nil {
		return ErrMemcachedNotinitialized
	}
	return c.client.Ping()
}
