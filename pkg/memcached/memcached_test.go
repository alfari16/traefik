//go:build integration
// +build integration

package memcached

import (
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/reconnectingmemcached"
	"testing"
	"time"
)

func TestMemcached(t *testing.T) {
	t.Skip()
	c := reconnectingmemcached.NewClient(reconnectingmemcached.Options{
		Servers:                      []string{"memcached-api-gateway.service.consul:11211"},
		MaxIdleConnections:           10,
		AllowedFailuresBeforeHealing: 3,
	})

	err := c.Ping()
	require.NoError(t, err)

	kv := "abogoboga"
	ok := c.Set(&memcache.Item{
		Key:        kv,
		Value:      []byte(kv),
		Expiration: int32(time.Duration(time.Second * 5).Seconds()),
	})
	if !ok {
		t.Error("failed to set cache")
	}

	list, ok := c.GetMulti([]string{"abogoboga"})
	if !ok {
		t.Error("failed to get cache")
	}
	if l := len(list); l != 1 {
		t.Errorf("list length doesnt match: %d", l)
	}

	item, ok := list[kv]
	if !ok {
		t.Errorf("key not found: %s", kv)
	}
	if kv != string(item.Value) {
		t.Errorf("value not match: %s", item.Value)
	}
}
