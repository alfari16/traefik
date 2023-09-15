package cache

import (
	"bytes"
	"net/http"
)

type loggedResponseWriter struct {
	http.ResponseWriter
	code   int
	body   *bytes.Buffer
	header http.Header
}

type cacheItem struct {
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

func (w *loggedResponseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *loggedResponseWriter) Header() http.Header {
	w.header = w.ResponseWriter.Header().Clone()
	return w.ResponseWriter.Header()
}

func (w *loggedResponseWriter) WriteHeader(code int) {
	w.ResponseWriter.WriteHeader(code)
	w.code = code
}
