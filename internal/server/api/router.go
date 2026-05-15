package api

import (
	"context"
	"log/slog"
	"time"

	"github.com/arcgolabs/authx"
	"github.com/arcgolabs/httpx"
	"github.com/arcgolabs/httpx/adapter"
	"github.com/arcgolabs/httpx/adapter/std"
	"github.com/arcgolabs/observabilityx"
	"github.com/lyonbrown4d/orivis/internal/server/config"
	"github.com/lyonbrown4d/orivis/internal/server/store"
	"github.com/lyonbrown4d/orivis/internal/shared/buildinfo"
)

type Server struct {
	cfg     config.Config
	logger  *slog.Logger
	store   *store.Store
	auth    *authx.Engine
	obs     observabilityx.Observability
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
	stdAdapter := std.New(nil, adapter.HumaOptions{
		Title:       "Orivis API",
		Version:     buildinfo.Version,
		Description: "Distributed availability observability API",
		DocsPath:    "/docs",
		OpenAPIPath: "/openapi",
	})

	runtime := httpx.New(
		httpx.WithAdapter(stdAdapter),
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
	return s.runtime.Shutdown()
}

func (s *Server) registerRoutes() {
	httpx.MustGet(s.runtime, "/", func(context.Context, *struct{}) (*metadataOutput, error) {
		out := &metadataOutput{}
		out.Body.Name = "orivis-server"
		out.Body.Env = s.cfg.App.Env
		out.Body.Version = buildinfo.Current()
		out.Body.Database.Driver = s.cfg.DB.Driver
		if s.store != nil && s.store.DB != nil && s.store.DB.Dialect() != nil {
			out.Body.Database.Dialect = s.store.DB.Dialect().Name()
		}
		return out, nil
	})

	httpx.MustGet(s.runtime, "/healthz", func(context.Context, *struct{}) (*statusOutput, error) {
		return newStatusOutput("ok"), nil
	})

	httpx.MustGet(s.runtime, "/readyz", func(context.Context, *struct{}) (*statusOutput, error) {
		return newStatusOutput("ready"), nil
	})

	httpx.MustGet(s.runtime, "/api/agent/tasks", func(context.Context, *agentTasksInput) (*agentTasksOutput, error) {
		out := &agentTasksOutput{}
		out.Body.Tasks = []agentTask{}
		return out, nil
	})

	httpx.MustPost(s.runtime, "/api/agent/results", func(context.Context, *agentResultsInput) (*statusOutput, error) {
		return newStatusOutput("accepted"), nil
	})
}

type metadataOutput struct {
	Body struct {
		Name     string         `json:"name"`
		Env      string         `json:"env"`
		Version  buildinfo.Info `json:"version"`
		Database struct {
			Driver  string `json:"driver"`
			Dialect string `json:"dialect,omitempty"`
		} `json:"database"`
	} `json:"body"`
}

type statusOutput struct {
	Body struct {
		Status string `json:"status"`
	} `json:"body"`
}

func newStatusOutput(status string) *statusOutput {
	out := &statusOutput{}
	out.Body.Status = status
	return out
}

type agentTasksInput struct{}

type agentTasksOutput struct {
	Body struct {
		Tasks []agentTask `json:"tasks"`
	} `json:"body"`
}

type agentTask struct {
	ID        string `json:"id"`
	MonitorID string `json:"monitor_id"`
	Type      string `json:"type"`
	Target    string `json:"target"`
}

type agentResultsInput struct {
	Body struct {
		AgentID   string `json:"agent_id"`
		MonitorID string `json:"monitor_id"`
		Status    string `json:"status"`
		CheckedAt string `json:"checked_at,omitempty"`
	} `json:"body"`
}
