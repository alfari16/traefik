
<p align="center">
<img src="docs/content/assets/img/traefik.logo.png" alt="Traefik" title="Traefik" />
</p>

[![Build Status SemaphoreCI](https://semaphoreci.com/api/v1/containous/traefik/branches/master/shields_badge.svg)](https://semaphoreci.com/containous/traefik)
[![Docs](https://img.shields.io/badge/docs-current-brightgreen.svg)](https://doc.traefik.io/traefik)
[![Go Report Card](https://goreportcard.com/badge/traefik/traefik)](https://goreportcard.com/report/traefik/traefik)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/traefik/traefik/blob/master/LICENSE.md)
[![Join the community support forum at https://community.traefik.io/](https://img.shields.io/badge/style-register-green.svg?style=social&label=Discourse)](https://community.traefik.io/)
[![Twitter](https://img.shields.io/twitter/follow/traefik.svg?style=social)](https://twitter.com/intent/follow?screen_name=traefik)

Traefik (pronounced _traffic_) is a modern HTTP reverse proxy and load balancer that makes deploying microservices easy.
This fork extends the open-source [Traefik](https://github.com/traefik/traefik) with enterprise-grade features, focusing on distributed capabilities for high-availability environments.

## ✨ Enhanced Enterprise Features

This distribution is based on Traefik 2.x, includes the following enterprise-grade features:

- **Distributed Rate Limiter**: Coordinate rate limiting across your entire Traefik cluster for consistent API protection
- **Distributed Cache**: Improve performance with a cluster-wide caching system
- **Coming Soon: Distributed Inflight Request Limiting**: Prevent system overload with coordinated request management

All distributed features are designed to work seamlessly in multi-node deployments without additional configuration.

## Prerequisites

To utilize the distributed features, you'll need:

- Memcached server(s) for cluster coordination
- Basic Traefik configuration with the following additions to your `traefik.yaml`:

```yaml
memcached:
  # Single node
  address: "localhost:11211"
  # Multiple nodes configuration soon
```

## New Additional Features

Checkout the examples [here](examples/).
  
### Distributed Cache

Enable cache on the endpoint. Configurations available:

- `ttl` — Cache ttl. Default to 300ms

- `variationHeaders` — Different value for these headers will have its own cache. For example if you cache user profile endpoint each authorization header/access token will be treated as different value.

> For conveniency and concistency, this module will only cache GET method and http 200 status. If the request is not GET or the response is not 200, it will not cached and forwarded directly to the service.
> This middleware has no cache invalidator except ttl. Use wisely
  
### Distributed Rate Limiter

Distributed rate limiter through cluster/replicas. Configuration available:

- `burst` required — total request allowed per ip

- `period` required — Golang `time.Duration` reff

### Coming Soon: Distributed Inflight Request Limit
  
- The same exact feature as of now, but distributed through cluster/replicas

## Supported Backends

Supported in all providers available in Traefik.


## Contributing

If you'd like to contribute to the project, refer to the [contributing documentation](CONTRIBUTING.md).

Please note that this project is released with a [Contributor Code of Conduct](CODE_OF_CONDUCT.md).
By participating in this project, you agree to abide by its terms.

