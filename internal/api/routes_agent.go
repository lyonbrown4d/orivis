package api

import (
	"context"
	"fmt"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/httpx"
	"github.com/danielgtaylor/huma/v2"
	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/protocol"
	"github.com/lyonbrown4d/orivis/internal/store"
)

func (s *Server) registerAgentRoutes() {
	s.registerAgentRegisterRoute()
	s.registerAgentHeartbeatRoute()
	s.registerAgentTasksRoute()
	s.registerAgentMonitorSyncRoute()
	s.registerAgentResultsRoute()
}

func (s *Server) registerAgentRegisterRoute() {
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
}

func (s *Server) registerAgentHeartbeatRoute() {
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
}

func (s *Server) registerAgentTasksRoute() {
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
		out.Body.Tasks = collectionlist.MapList(
			collectionlist.NewList(monitors...),
			func(_ int, monitor model.Monitor) protocol.AgentTask {
				return protocol.AgentTask{
					ID:              monitor.ID,
					MonitorID:       monitor.ID,
					Type:            string(monitor.Type),
					Target:          monitor.Target,
					IntervalSeconds: int(monitor.Interval.Seconds()),
					TimeoutSeconds:  int(monitor.Timeout.Seconds()),
				}
			},
		).Values()
		return out, nil
	})
}

func (s *Server) registerAgentMonitorSyncRoute() {
	httpx.MustPost(s.runtime, "/api/agent/monitors", func(ctx context.Context, input *agentMonitorSyncInput) (*agentMonitorSyncOutput, error) {
		if s.store == nil || s.store.AgentStore() == nil || s.store.MonitorStore() == nil {
			return nil, huma.Error500InternalServerError("agent monitor stores are not available")
		}

		agent, err := s.store.AgentStore().Authenticate(ctx, input.Body.AgentID, input.Body.Token)
		if err != nil {
			return nil, apiError(err)
		}

		synced, err := s.syncAgentMonitors(ctx, agent, input.Body.Monitors)
		if err != nil {
			return nil, apiError(err)
		}

		out := &agentMonitorSyncOutput{}
		out.Body.Synced = synced
		return out, nil
	})
}

func (s *Server) syncAgentMonitors(ctx context.Context, agent model.Agent, monitors []protocol.AgentDiscoveredMonitor) (int, error) {
	synced := 0
	for index := range monitors {
		if err := s.syncAgentMonitor(ctx, agent, monitors[index]); err != nil {
			return 0, err
		}
		synced++
	}
	return synced, nil
}

func (s *Server) syncAgentMonitor(ctx context.Context, agent model.Agent, discovered protocol.AgentDiscoveredMonitor) error {
	environmentID, err := s.store.EnvironmentIDForAgent(ctx, agent, discovered.EnvironmentCode)
	if err != nil {
		return fmt.Errorf("resolve discovered monitor environment: %w", err)
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
		return fmt.Errorf("upsert discovered monitor: %w", err)
	}
	if err := s.store.MonitorStore().AssignAgent(ctx, monitor.ID, agent.ID); err != nil {
		return fmt.Errorf("assign discovered monitor: %w", err)
	}
	return nil
}

func (s *Server) registerAgentResultsRoute() {
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
