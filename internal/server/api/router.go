package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/arcgolabs/authx"
	"github.com/arcgolabs/httpx"
	"github.com/arcgolabs/httpx/adapter"
	adapterfiber "github.com/arcgolabs/httpx/adapter/fiber"
	"github.com/arcgolabs/observabilityx"
	"github.com/danielgtaylor/huma/v2"
	"github.com/lyonbrown4d/orivis/internal/server/config"
	"github.com/lyonbrown4d/orivis/internal/server/store"
	"github.com/lyonbrown4d/orivis/internal/shared/buildinfo"
	"github.com/lyonbrown4d/orivis/internal/shared/model"
	"github.com/lyonbrown4d/orivis/internal/shared/protocol"
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
	fiberAdapter := adapterfiber.New(nil, adapter.HumaOptions{
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
	httpx.MustGet(s.runtime, "/", func(ctx context.Context, input *dashboardInput) (*dashboardOutput, error) {
		if err := s.verifyDashboardAuth(input.Authorization); err != nil {
			return nil, err
		}
		html, err := s.renderDashboard(ctx)
		if err != nil {
			return nil, err
		}
		return &dashboardOutput{
			ContentType: "text/html; charset=utf-8",
			Body:        html,
		}, nil
	})

	httpx.MustGet(s.runtime, "/api/server/metadata", func(context.Context, *struct{}) (*metadataOutput, error) {
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

	httpx.MustPost(s.runtime, "/api/agent/register", func(ctx context.Context, input *agentRegisterInput) (*agentRegisterOutput, error) {
		if err := s.verifyBootstrapToken(input.Body.Token); err != nil {
			return nil, apiError(err)
		}
		if s.store == nil || s.store.AgentStore() == nil {
			return nil, huma.Error500InternalServerError("agent store is not available")
		}

		agent, err := s.store.AgentStore().Register(ctx, store.RegisterAgentParams{
			Name:             input.Body.Name,
			Token:            input.Body.Token,
			RegionCode:       input.Body.RegionCode,
			EnvironmentCodes: input.Body.EnvironmentCodes,
			RuntimeType:      input.Body.RuntimeType,
			Version:          input.Body.Version,
		})
		if err != nil {
			return nil, apiError(err)
		}

		out := &agentRegisterOutput{}
		out.Body.AgentID = agent.ID
		out.Body.RegionID = agent.RegionID
		out.Body.Status = string(agent.Status)
		out.Body.ServerTime = time.Now().UTC()
		return out, nil
	})

	httpx.MustPost(s.runtime, "/api/agent/heartbeat", func(ctx context.Context, input *agentHeartbeatInput) (*agentHeartbeatOutput, error) {
		if s.store == nil || s.store.AgentStore() == nil {
			return nil, huma.Error500InternalServerError("agent store is not available")
		}

		agent, err := s.store.AgentStore().RecordHeartbeat(ctx, store.AgentHeartbeatParams{
			AgentID: input.Body.AgentID,
			Token:   input.Body.Token,
			Version: input.Body.Version,
			SeenAt:  input.Body.SentAt,
		})
		if err != nil {
			return nil, apiError(err)
		}

		out := &agentHeartbeatOutput{}
		out.Body.AgentID = agent.ID
		out.Body.Status = string(agent.Status)
		out.Body.ServerTime = time.Now().UTC()
		return out, nil
	})

	httpx.MustGet(s.runtime, "/api/agent/tasks", func(ctx context.Context, input *agentTasksInput) (*agentTasksOutput, error) {
		if s.store == nil || s.store.AgentStore() == nil || s.store.MonitorStore() == nil {
			return nil, huma.Error500InternalServerError("agent task stores are not available")
		}

		agent, err := s.store.AgentStore().Authenticate(ctx, input.AgentID, input.Token)
		if err != nil {
			return nil, apiError(err)
		}
		monitors, err := s.store.MonitorStore().ListAssignedEnabled(ctx, agent.ID)
		if err != nil {
			return nil, apiError(err)
		}

		out := &agentTasksOutput{}
		out.Body.Tasks = make([]protocol.AgentTask, 0, len(monitors))
		for _, monitor := range monitors {
			out.Body.Tasks = append(out.Body.Tasks, protocol.AgentTask{
				ID:              monitor.ID,
				MonitorID:       monitor.ID,
				Type:            string(monitor.Type),
				Target:          monitor.Target,
				IntervalSeconds: int(monitor.Interval.Seconds()),
				TimeoutSeconds:  int(monitor.Timeout.Seconds()),
			})
		}
		return out, nil
	})

	httpx.MustPost(s.runtime, "/api/agent/monitors", func(ctx context.Context, input *agentMonitorSyncInput) (*agentMonitorSyncOutput, error) {
		if s.store == nil || s.store.AgentStore() == nil || s.store.MonitorStore() == nil {
			return nil, huma.Error500InternalServerError("agent monitor stores are not available")
		}

		agent, err := s.store.AgentStore().Authenticate(ctx, input.Body.AgentID, input.Body.Token)
		if err != nil {
			return nil, apiError(err)
		}

		synced := 0
		for _, discovered := range input.Body.Monitors {
			environmentID, err := s.store.EnvironmentIDForAgent(ctx, agent, discovered.EnvironmentCode)
			if err != nil {
				return nil, apiError(err)
			}
			monitor, err := s.store.MonitorStore().UpsertDiscovered(ctx, store.UpsertDiscoveredMonitorParams{
				SourceKey:         discovered.SourceKey,
				Name:              discovered.Name,
				Type:              model.MonitorType(normalizeProtocolString(discovered.Type)),
				Target:            discovered.Target,
				EnvironmentID:     environmentID,
				Enabled:           protocolEnabled(discovered.Enabled),
				Interval:          time.Duration(discovered.IntervalSeconds) * time.Second,
				Timeout:           time.Duration(discovered.TimeoutSeconds) * time.Second,
				RetryCount:        discovered.RetryCount,
				AggregationPolicy: model.AggregationPolicy(normalizeProtocolString(discovered.AggregationPolicy)),
			})
			if err != nil {
				return nil, apiError(err)
			}
			if err := s.store.MonitorStore().AssignAgent(ctx, monitor.ID, agent.ID); err != nil {
				return nil, apiError(err)
			}
			synced++
		}

		out := &agentMonitorSyncOutput{}
		out.Body.Synced = synced
		return out, nil
	})

	httpx.MustPost(s.runtime, "/api/agent/results", func(ctx context.Context, input *agentResultsInput) (*statusOutput, error) {
		if s.store == nil || s.store.AgentStore() == nil || s.store.ResultStore() == nil {
			return nil, huma.Error500InternalServerError("agent result stores are not available")
		}

		agent, err := s.store.AgentStore().Authenticate(ctx, input.Body.AgentID, input.Body.Token)
		if err != nil {
			return nil, apiError(err)
		}
		if _, err := s.store.ResultStore().Record(ctx, store.RecordProbeResultParams{
			Agent:        agent,
			MonitorID:    input.Body.MonitorID,
			Status:       modelStatus(input.Body.Status),
			Latency:      time.Duration(input.Body.LatencyMS) * time.Millisecond,
			ErrorMessage: input.Body.ErrorMessage,
			CheckedAt:    input.Body.CheckedAt,
			RawDetail:    input.Body.RawDetail,
		}); err != nil {
			return nil, apiError(err)
		}

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

type agentRegisterInput struct {
	Body protocol.AgentRegisterRequest `json:"body"`
}

type agentRegisterOutput struct {
	Body protocol.AgentRegisterResponse `json:"body"`
}

type agentHeartbeatInput struct {
	Body protocol.AgentHeartbeatRequest `json:"body"`
}

type agentHeartbeatOutput struct {
	Body protocol.AgentHeartbeatResponse `json:"body"`
}

type agentTasksInput struct {
	AgentID string `query:"agent_id" validate:"required"`
	Token   string `query:"token,omitempty"`
}

type agentTasksOutput struct {
	Body protocol.AgentTasksResponse `json:"body"`
}

type agentMonitorSyncInput struct {
	Body protocol.AgentMonitorSyncRequest `json:"body"`
}

type agentMonitorSyncOutput struct {
	Body protocol.AgentMonitorSyncResponse `json:"body"`
}

type agentResultsInput struct {
	Body protocol.AgentResultRequest `json:"body"`
}

func (s *Server) verifyBootstrapToken(token string) error {
	expected := strings.TrimSpace(s.cfg.Auth.Agent.Token)
	if expected == "" {
		return nil
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
		return store.ErrUnauthorized
	}
	return nil
}

func apiError(err error) error {
	switch {
	case errors.Is(err, store.ErrInvalidInput):
		return huma.Error400BadRequest(err.Error(), err)
	case errors.Is(err, store.ErrUnauthorized):
		return huma.Error401Unauthorized("unauthorized", err)
	case errors.Is(err, store.ErrNotFound):
		return huma.Error404NotFound(err.Error(), err)
	default:
		return huma.Error500InternalServerError("internal server error", err)
	}
}

func modelStatus(value string) model.Status {
	return model.Status(normalizeProtocolString(value))
}

func normalizeProtocolString(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func protocolEnabled(value *bool) bool {
	if value == nil {
		return true
	}
	return *value
}
