package api

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/arcgolabs/authx"
	"github.com/arcgolabs/httpx"
	"github.com/arcgolabs/httpx/adapter"
	adapterfiber "github.com/arcgolabs/httpx/adapter/fiber"
	"github.com/arcgolabs/observabilityx"
	"github.com/gofiber/fiber/v2"
	"github.com/lyonbrown4d/orivis/internal/buildinfo"
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/lyonbrown4d/orivis/internal/store"
)

type Server struct {
	cfg     config.Config
	logger  *slog.Logger
	store   *store.Store
	auth    *authx.Engine
	obs     observabilityx.Observability
	app     *fiber.App
	runtime httpx.ServerRuntime
	errCh   chan error
}

func NewServer(
	cfg config.Config,
	logger *slog.Logger,
	storage *store.Store,
	auth *authx.Engine,
	obs observabilityx.Observability,
) *Server {
	app := fiber.New(fiber.Config{
		Views: newDashboardViews(),
	})

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
		auth:    auth,
		obs:     observabilityx.Normalize(obs, logger),
		app:     app,
		runtime: runtime,
		errCh:   make(chan error, 1),
	}
	server.registerRoutes()
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

func (s *Server) registerRoutes() {
	s.registerDashboardRoutes()
	s.registerMetadataRoutes()
	s.registerHealthRoutes()
	s.registerAgentRoutes()
}
