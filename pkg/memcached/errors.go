package memcached

import (
	"errors"
	"fmt"
)

var ErrMemcachedNotinitialized = errors.New("memcached not initialized")
var ErrUnknown = errors.New("memcached unknown error")

type ErrKeyNotFound struct {
	key string
}

func (e ErrKeyNotFound) Error() string {
	return fmt.Sprintf("memcached key not found: %s", e.key)
}
