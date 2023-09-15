package memcached

import (
	"github.com/traefik/traefik/v2/pkg/config/static"
	"go.skia.org/infra/go/reconnectingmemcached"
)

type Client struct {
	client reconnectingmemcached.Client
}

func NewMemcachedClient(conf *static.Memcached) *Client {
	if conf == nil {
		return nil
	}
	c := reconnectingmemcached.NewClient(reconnectingmemcached.Options{
		Servers:                      []string{conf.Address},
		MaxIdleConnections:           10,
		AllowedFailuresBeforeHealing: 3,
	})
	return &Client{
		client: c,
	}
}
