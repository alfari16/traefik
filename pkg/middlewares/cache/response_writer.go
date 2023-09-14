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
