package server

import (
	"context"
	"github.com/traefik/traefik/v2/pkg/config/runtime"
	"github.com/traefik/traefik/v2/pkg/config/static"
	"github.com/traefik/traefik/v2/pkg/log"
	"github.com/traefik/traefik/v2/pkg/memcached"
	"github.com/traefik/traefik/v2/pkg/metrics"
	"github.com/traefik/traefik/v2/pkg/server/middleware"
	tcpmiddleware "github.com/traefik/traefik/v2/pkg/server/middleware/tcp"
	"github.com/traefik/traefik/v2/pkg/server/router"
	tcprouter "github.com/traefik/traefik/v2/pkg/server/router/tcp"
	udprouter "github.com/traefik/traefik/v2/pkg/server/router/udp"
	"github.com/traefik/traefik/v2/pkg/server/service"
	"github.com/traefik/traefik/v2/pkg/server/service/tcp"
	"github.com/traefik/traefik/v2/pkg/server/service/udp"
	"github.com/traefik/traefik/v2/pkg/tls"
	udptypes "github.com/traefik/traefik/v2/pkg/udp"
)

// RouterFactory the factory of TCP/UDP routers.
type RouterFactory struct {
	entryPointsTCP []string
	entryPointsUDP []string

	managerFactory  *service.ManagerFactory
	metricsRegistry metrics.Registry

	pluginBuilder middleware.PluginsBuilder

	chainBuilder *middleware.ChainBuilder
	tlsManager   *tls.Manager

	memcached *memcached.Client
}

// NewRouterFactory creates a new RouterFactory.
func NewRouterFactory(staticConfiguration static.Configuration, managerFactory *service.ManagerFactory, tlsManager *tls.Manager,
	chainBuilder *middleware.ChainBuilder, pluginBuilder middleware.PluginsBuilder, metricsRegistry metrics.Registry, memcached *memcached.Client,
) *RouterFactory {
	var entryPointsTCP, entryPointsUDP []string
	for name, cfg := range staticConfiguration.EntryPoints {
		protocol, err := cfg.GetProtocol()
		if err != nil {
			// Should never happen because Traefik should not start if protocol is invalid.
			log.WithoutContext().Errorf("Invalid protocol: %v", err)
		}

		if protocol == "udp" {
			entryPointsUDP = append(entryPointsUDP, name)
		} else {
			entryPointsTCP = append(entryPointsTCP, name)
		}
	}

	return &RouterFactory{
		entryPointsTCP:  entryPointsTCP,
		entryPointsUDP:  entryPointsUDP,
		managerFactory:  managerFactory,
		metricsRegistry: metricsRegistry,
		tlsManager:      tlsManager,
		chainBuilder:    chainBuilder,
		pluginBuilder:   pluginBuilder,
		memcached:       memcached,
	}
}

// CreateRouters creates new TCPRouters and UDPRouters.
func (f *RouterFactory) CreateRouters(rtConf *runtime.Configuration) (map[string]*tcprouter.Router, map[string]udptypes.Handler) {
	ctx := context.Background()

	// HTTP
	serviceManager := f.managerFactory.Build(rtConf)

	middlewaresBuilder := middleware.NewBuilder(rtConf.Middlewares, serviceManager, f.pluginBuilder, f.memcached)

	routerManager := router.NewManager(rtConf, serviceManager, middlewaresBuilder, f.chainBuilder, f.metricsRegistry)

	handlersNonTLS := routerManager.BuildHandlers(ctx, f.entryPointsTCP, false)
	handlersTLS := routerManager.BuildHandlers(ctx, f.entryPointsTCP, true)

	serviceManager.LaunchHealthCheck()

	// TCP
	svcTCPManager := tcp.NewManager(rtConf)

	middlewaresTCPBuilder := tcpmiddleware.NewBuilder(rtConf.TCPMiddlewares)

	rtTCPManager := tcprouter.NewManager(rtConf, svcTCPManager, middlewaresTCPBuilder, handlersNonTLS, handlersTLS, f.tlsManager)
	routersTCP := rtTCPManager.BuildHandlers(ctx, f.entryPointsTCP)

	// UDP
	svcUDPManager := udp.NewManager(rtConf)
	rtUDPManager := udprouter.NewManager(rtConf, svcUDPManager)
	routersUDP := rtUDPManager.BuildHandlers(ctx, f.entryPointsUDP)

	rtConf.PopulateUsedBy()

	return routersTCP, routersUDP
}
