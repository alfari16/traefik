// Package ratelimiter implements a burstRate limiting and traffic shaping middleware with a set of token buckets.
package ratelimiter

import (
	"context"
	"errors"
	"fmt"
	"github.com/mailgun/ttlmap"
	mc "github.com/traefik/traefik/v2/pkg/memcached"
	"math"
	"net/http"
	"time"

	"github.com/opentracing/opentracing-go/ext"
	"github.com/traefik/traefik/v2/pkg/config/dynamic"
	"github.com/traefik/traefik/v2/pkg/log"
	"github.com/traefik/traefik/v2/pkg/middlewares"
	"github.com/traefik/traefik/v2/pkg/tracing"
	"github.com/vulcand/oxy/utils"
	"golang.org/x/time/rate"
)

const (
	typeName   = "RateLimiter"
	maxSources = 65536
)

// rateLimiter implements burstRate limiting and traffic shaping with a set of token buckets;
// one for each traffic source. The same parameters are applied to all the buckets.
type rateLimiter struct {
	name      string
	burstRate *rate.Limiter // reqs/s
	burst     int64
	average   int64
	// maxDelay is the maximum duration we're willing to wait for a bucket reservation to become effective, in nanoseconds.
	// For now it is somewhat arbitrarily set to 1/(2*burstRate).
	maxDelay time.Duration
	// each burstRate limiter for a given source is stored in the buckets ttlmap.
	// To keep this ttlmap constrained in size,
	// each ratelimiter is "garbage collected" when it is considered expired.
	// It is considered expired after it hasn't been used for ttl seconds.
	ttl           time.Duration
	sourceMatcher utils.SourceExtractor
	next          http.Handler

	buckets *ttlmap.TtlMap // actual buckets, keyed by source.
	mh      middlewares.IMemcachedHandler[cacheItem]
}

type cacheItem struct {
	StoredAt int64
	Counter  int64
}

// New returns a burstRate limiter middleware.
func New(ctx context.Context, next http.Handler, config dynamic.RateLimit, name string, memcached *mc.Client) (http.Handler, error) {
	ctxLog := log.With(ctx, log.Str(log.MiddlewareName, name), log.Str(log.MiddlewareType, typeName))
	log.FromContext(ctxLog).Infof("Creating middleware with period:%s", config.Period.String())

	mh := mc.NewMemcachedHandler[cacheItem](memcached)

	if config.SourceCriterion == nil ||
		config.SourceCriterion.IPStrategy == nil &&
			config.SourceCriterion.RequestHeaderName == "" && !config.SourceCriterion.RequestHost {
		config.SourceCriterion = &dynamic.SourceCriterion{
			IPStrategy: &dynamic.IPStrategy{},
		}
	}

	sourceMatcher, err := middlewares.GetSourceExtractor(ctxLog, config.SourceCriterion)
	if err != nil {
		return nil, err
	}

	period := time.Duration(config.Period)
	if period.Seconds() < 0 {
		return nil, fmt.Errorf("negative value not valid for period: %v", period)
	}
	if period.Seconds() == 0 {
		period = time.Second
	}

	return &rateLimiter{
		name:          name,
		average:       config.Average,
		next:          next,
		sourceMatcher: sourceMatcher,
		ttl:           period,
		mh:            mh,
	}, nil
}

func (rl *rateLimiter) GetTracingInformation() (string, ext.SpanKindEnum) {
	return rl.name, tracing.SpanKindNoneEnum
}

func (rl *rateLimiter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := middlewares.GetLoggerCtx(r.Context(), rl.name, typeName)
	logger := log.FromContext(ctx)

	source, amount, err := rl.sourceMatcher.Extract(r)
	if err != nil {
		logger.Errorf("could not extract source of request: %v", err)
		http.Error(w, "could not extract source of request", http.StatusInternalServerError)
		return
	}
	compositeSources := fmt.Sprintf("rl:%s%s%s%s", source, rl.name, r.Method, r.URL.Path)

	if rl.average == 0 {
		rl.next.ServeHTTP(w, r)
		return
	}

	now := time.Now().Unix()
	ci := cacheItem{
		StoredAt: now,
		Counter:  0,
	}
	if err := rl.mh.Get(r.Context(), compositeSources, &ci); err != nil && !errors.As(err, &mc.ErrKeyNotFound{}) {
		logger.Errorf("could not get from memcached: %v, skipping rate limit", err)
		rl.next.ServeHTTP(w, r)
		return
	}

	ttl := time.Since(time.Unix(ci.StoredAt, 0).Add(rl.ttl))
	ci.Counter = ci.Counter + amount

	logger.Debugf("stored_at:%s source:%s amount:%d rl_ttl:%s ttl:%s counter:%d is_now:%t", time.Unix(ci.StoredAt, 0).String(), compositeSources, amount, rl.ttl.String(), ttl.String(), ci.Counter, ci.StoredAt == now)
	if ci.Counter > rl.average && ttl.Seconds() < 0 {
		rl.serveDelayError(ctx, w, ttl)
		return
	}

	// We Set even in the case where the source already exists,
	// because we want to update the expiryTime everytime we get the source,
	// as the expiryTime is supposed to reflect the activity (or lack thereof) on that source.
	if err := rl.mh.Set(r.Context(), compositeSources, ci, -ttl); err != nil {
		logger.Errorf("could not insert/update to memcached: %v", err)
	}

	rl.next.ServeHTTP(w, r)
}

func (rl *rateLimiter) serveDelayError(ctx context.Context, w http.ResponseWriter, delay time.Duration) {
	w.Header().Set("Retry-After", fmt.Sprintf("%.0f", math.Ceil(delay.Seconds())))
	w.Header().Set("X-Retry-In", delay.String())
	w.WriteHeader(http.StatusTooManyRequests)

	if _, err := w.Write([]byte(http.StatusText(http.StatusTooManyRequests))); err != nil {
		log.FromContext(ctx).Errorf("could not serve 429: %v", err)
	}
}
