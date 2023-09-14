package cache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/traefik/traefik/v2/pkg/config/dynamic"
	"github.com/traefik/traefik/v2/pkg/log"
	"github.com/traefik/traefik/v2/pkg/memcached"
	"github.com/traefik/traefik/v2/pkg/middlewares"
	"github.com/traefik/traefik/v2/pkg/tracing"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	typeName = "Cache"
)

type cache struct {
	next             http.Handler
	name             string
	memcached        memcached.IMemcached
	ttl              time.Duration
	variationHeaders map[string]interface{}
}

func New(ctx context.Context, next http.Handler, conf dynamic.Cache, name string, memcached memcached.IMemcached) (http.Handler, error) {
	log.FromContext(middlewares.GetLoggerCtx(ctx, name, typeName)).Infof("Creating middleware with: %s - %s", name, conf.TTL, conf.VariationHeaders)

	ttl, err := time.ParseDuration(conf.TTL)
	if err != nil {
		log.FromContext(middlewares.GetLoggerCtx(context.Background(), name, typeName)).Error(err)
		return nil, err
	}

	if err := memcached.Ping(); err != nil {
		log.FromContext(middlewares.GetLoggerCtx(context.Background(), name, typeName)).Error(err)
		return nil, err
	}

	variationHeaders := make(map[string]interface{})
	for _, header := range strings.Split(conf.VariationHeaders, ",") {
		variationHeaders[header] = nil
	}

	return &cache{
		next:             next,
		name:             name,
		memcached:        memcached,
		ttl:              ttl,
		variationHeaders: variationHeaders,
	}, nil
}

func (p *cache) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cacheKey := p.buildKey(r)

	containsNoCache := strings.Contains(r.Header.Get("cache-control"), "no-cache")
	isGetMethod := r.Method == http.MethodGet
	isCachable := !containsNoCache && isGetMethod
	if isCachable {
		item, err := p.memcached.Get(r.Context(), cacheKey)
		if err == nil {
			p.serveFromCache(w, item)
			return
		}
	}

	ww := &loggedResponseWriter{ResponseWriter: w, body: new(bytes.Buffer)}
	p.next.ServeHTTP(ww, r)

	isResponseOk := ww.code == http.StatusOK
	isCachable = isCachable && isResponseOk
	if isCachable {
		go func() {
			ww.header.Del("cache-control")
			ww.header.Del("age")
			item := memcached.CacheItem{
				Body:     ww.body.Bytes(),
				Status:   ww.code,
				Header:   ww.header,
				StoredAt: time.Now().UTC().Unix(),
				MaxAge:   int64(p.ttl.Seconds()),
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := p.memcached.Set(ctx, cacheKey, item, p.ttl); err != nil {
				log.FromContext(middlewares.GetLoggerCtx(context.Background(), p.name, typeName)).Error(err)
			}

			log.FromContext(middlewares.GetLoggerCtx(context.Background(), p.name, typeName)).Info("set to cache")
		}()
	}
}

func (p *cache) GetTracingInformation() (string, ext.SpanKindEnum) {
	return p.name, tracing.SpanKindNoneEnum
}

func (p *cache) buildKey(r *http.Request) string {
	headers := make([]string, 0)
	for key, values := range r.Header {
		key = strings.ToLower(key)
		if _, included := p.variationHeaders[key]; !included {
			continue
		}

		headers = append(headers, strings.Join(values, ":"))
	}

	baseKey := p.name + ";" + r.RequestURI + ";" + strings.Join(headers, ",")
	cacheKey := sha256.Sum256([]byte(baseKey))

	return hex.EncodeToString(cacheKey[:])
}

func (p *cache) serveFromCache(w http.ResponseWriter, item memcached.CacheItem) {
	for key, values := range item.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.Header().Set("age", strconv.FormatInt(item.Age, 10))
	w.Header().Set("cache-control", "max-age="+strconv.FormatInt(item.MaxAge, 10))

	log.FromContext(middlewares.GetLoggerCtx(context.Background(), p.name, typeName)).Info("serve from cache")

	w.WriteHeader(item.Status)
	_, err := w.Write(item.Body)
	if err != nil {
		log.FromContext(middlewares.GetLoggerCtx(context.Background(), p.name, typeName)).Error(err)
	}
}
