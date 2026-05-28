package api

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/arcgolabs/authx"
	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/httpx"
	"github.com/arcgolabs/httpx/adapter"
	adapterfiber "github.com/arcgolabs/httpx/adapter/fiber"
	"github.com/arcgolabs/observabilityx"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/helmet"
	fiberrecover "github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/lyonbrown4d/orivis/internal/buildinfo"
	cachex "github.com/lyonbrown4d/orivis/internal/cache"
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/lyonbrown4d/orivis/internal/store"
)

type Server struct {
	cfg     config.Config
	logger  *slog.Logger
	store   *store.Store
	auth    *authx.Engine
	cache   cachex.Store
	obs     observabilityx.Observability
	app     *fiber.App
	runtime httpx.ServerRuntime
	errCh   chan error
}

type ServerRuntimeDeps struct {
	Auth  *authx.Engine
	Obs   observabilityx.Observability
	Cache cachex.Store
}

func NewServerRuntimeDeps(
	auth *authx.Engine,
	obs observabilityx.Observability,
	cacheStore cachex.Store,
) ServerRuntimeDeps {
	return ServerRuntimeDeps{
		Auth:  auth,
		Obs:   obs,
		Cache: cacheStore,
	}
}

func NewServer(
	cfg config.Config,
	logger *slog.Logger,
	storage *store.Store,
	deps ServerRuntimeDeps,
	endpoints *collectionlist.List[httpx.Endpoint],
) *Server {
	bodyLimit := httpBodyLimit(cfg)
	app := fiber.New(fiber.Config{
		BodyLimit: bodyLimit,
	})
	app.Use(requestid.New())
	app.Use(fiberrecover.New())
	app.Use(helmet.New(helmet.Config{
		CrossOriginEmbedderPolicy: "unsafe-none",
		CrossOriginResourcePolicy: "cross-origin",
	}))
	app.Use(gzipRequestMiddleware(bodyLimit))
	app.Use(compress.New())

	fiberAdapter := adapterfiber.New(app, adapter.HumaOptions{
		Title:       "Orivis API",
		Version:     buildinfo.Version,
		Description: "Distributed availability observability API",
		DocsPath:    "/docs",
		OpenAPIPath: "/openapi",
	})

	runtime := httpx.New(
		httpx.WithAdapter(fiberAdapter),
		httpx.WithLogger(logger),
		httpx.WithValidation(),
		httpx.WithAccessLog(true),
		httpx.WithPrintRoutes(cfg.App.Env != "production"),
	)

	server := &Server{
		cfg:     cfg,
		logger:  logger,
		store:   storage,
		auth:    deps.Auth,
		cache:   deps.Cache,
		obs:     observabilityx.Normalize(deps.Obs, logger),
		app:     app,
		runtime: runtime,
		errCh:   make(chan error, 1),
	}
	server.registerEndpoints(endpoints)
	server.registerDashboardRoutes()
	return server
}

func (s *Server) Runtime() httpx.ServerRuntime {
	return s.runtime
}

func (s *Server) Start(context.Context) error {
	go func() {
		s.logger.Info("starting http server", "addr", s.cfg.HTTP.Addr)
		s.errCh <- s.runtime.ListenAndServe(s.cfg.HTTP.Addr)
	}()

	select {
	case err := <-s.errCh:
		return err
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

func (s *Server) Stop(context.Context) error {
	if err := s.runtime.Shutdown(); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}
	return nil
}

func (s *Server) registerEndpoints(endpoints *collectionlist.List[httpx.Endpoint]) {
	if endpoints == nil {
		return
	}
	endpoints.Range(func(_ int, endpoint httpx.Endpoint) bool {
		s.runtime.RegisterOnly(endpoint)
		return true
	})
}
