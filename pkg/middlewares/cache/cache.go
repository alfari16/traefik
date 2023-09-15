package cache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/traefik/traefik/v2/pkg/config/dynamic"
	"github.com/traefik/traefik/v2/pkg/log"
	mc "github.com/traefik/traefik/v2/pkg/memcached"
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
	mh               middlewares.IMemcachedHandler[cacheItem]
	ttl              time.Duration
	variationHeaders map[string]interface{}
}

func New(ctx context.Context, next http.Handler, conf dynamic.Cache, name string, memcached *mc.Client) (http.Handler, error) {
	log.FromContext(middlewares.GetLoggerCtx(ctx, name, typeName)).Infof("Creating middleware with: %s - %s", name, conf.TTL, conf.VariationHeaders)

	ttl, err := time.ParseDuration(conf.TTL)
	if err != nil {
		log.FromContext(middlewares.GetLoggerCtx(context.Background(), name, typeName)).Error(err)
		return nil, err
	}

	mh := mc.NewMemcachedHandler[cacheItem](memcached)

	variationHeaders := make(map[string]interface{})
	for _, header := range strings.Split(conf.VariationHeaders, ",") {
		variationHeaders[header] = nil
	}

	return &cache{
		next:             next,
		name:             name,
		mh:               mh,
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
		var ci cacheItem
		err := p.mh.Get(r.Context(), cacheKey, &ci)
		if err == nil {
			ci.Age = int64(time.Since(time.Unix(ci.StoredAt, 0)).Seconds())
			p.serveFromCache(w, ci)
			return
		} else if !errors.As(err, &mc.ErrKeyNotFound{}) {
			log.FromContext(middlewares.GetLoggerCtx(context.Background(), p.name, typeName)).Error(err)
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
			item := cacheItem{
				Body:     ww.body.Bytes(),
				Status:   ww.code,
				Header:   ww.header,
				StoredAt: time.Now().Unix(),
				MaxAge:   int64(p.ttl.Seconds()),
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := p.mh.Set(ctx, cacheKey, item, p.ttl); err != nil {
				log.FromContext(middlewares.GetLoggerCtx(context.Background(), p.name, typeName)).Error(err)
			}

			log.FromContext(middlewares.GetLoggerCtx(context.Background(), p.name, typeName)).Debug("set to cache")
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

func (p *cache) serveFromCache(w http.ResponseWriter, item cacheItem) {
	for key, values := range item.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.Header().Set("age", strconv.FormatInt(item.Age, 10))
	w.Header().Set("cache-control", "max-age="+strconv.FormatInt(item.MaxAge, 10))

	log.FromContext(middlewares.GetLoggerCtx(context.Background(), p.name, typeName)).Debug("serve from cache")

	w.WriteHeader(item.Status)
	_, err := w.Write(item.Body)
	if err != nil {
		log.FromContext(middlewares.GetLoggerCtx(context.Background(), p.name, typeName)).Error(err)
	}
}
