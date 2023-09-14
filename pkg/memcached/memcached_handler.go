package memcached

import (
	"bytes"
	"context"
	"encoding/gob"
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/traefik/traefik/v2/pkg/config/static"
	"go.skia.org/infra/go/reconnectingmemcached"
	"time"
)

type Memcached struct {
	client reconnectingmemcached.Client
}

func NewMemcached(conf *static.Memcached) *Memcached {
	if conf == nil {
		return nil
	}
	c := reconnectingmemcached.NewClient(reconnectingmemcached.Options{
		Servers:                      []string{conf.Address},
		MaxIdleConnections:           10,
		AllowedFailuresBeforeHealing: 3,
	})
	return &Memcached{
		client: c,
	}
}

func (c *Memcached) Get(ctx context.Context, key string) (CacheItem, error) {
	if c.client == nil {
		return CacheItem{}, ErrMemcachedNotinitialized
	}

	items, ok := c.client.GetMulti([]string{key})
	if !ok {
		return CacheItem{}, ErrUnknown
	}

	item, ok := items[key]
	if !ok {
		return CacheItem{}, &ErrKeyNotFound{key: key}
	}

	buf := bytes.NewBuffer(item.Value)

	var i CacheItem
	if err := gob.NewDecoder(buf).Decode(&i); err != nil {
		return CacheItem{}, err
	}

	i.Age = int64(time.Since(time.Unix(i.StoredAt, 0)).Seconds())

	return i, nil
}

func (c *Memcached) Set(ctx context.Context, key string, item CacheItem, ttl time.Duration) error {
	if c.client == nil {
		return ErrMemcachedNotinitialized
	}

	buf := bytes.NewBuffer(nil)
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

func (c *Memcached) Ping() error {
	if c.client == nil {
		return ErrMemcachedNotinitialized
	}
	return c.client.Ping()
}
