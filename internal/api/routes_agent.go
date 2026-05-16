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

func (e *agentEndpoint) Register(registrar httpx.Registrar) {
	scope := registrar.Scope()
	httpx.MustGroupPost(scope, "api/agent/register", e.registerAgent)
	httpx.MustGroupPost(scope, "api/agent/heartbeat", e.heartbeat)
	httpx.MustGroupGet(scope, "api/agent/tasks", e.tasks)
	httpx.MustGroupPost(scope, "api/agent/monitors", e.syncMonitors)
	httpx.MustGroupPost(scope, "api/agent/results", e.reportResult)
}

func (e *agentEndpoint) registerAgent(ctx context.Context, input *agentRegisterInput) (*agentRegisterOutput, error) {
	if err := e.verifyBootstrapToken(input.Body.Token); err != nil {
		return nil, apiError(err)
	}
	if e.store == nil || e.store.AgentStore() == nil {
		return nil, huma.Error500InternalServerError("agent store is not available")
	}

	agent, err := e.store.AgentStore().Register(ctx, store.RegisterAgentParams{
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
}

func (e *agentEndpoint) heartbeat(ctx context.Context, input *agentHeartbeatInput) (*agentHeartbeatOutput, error) {
	if e.store == nil || e.store.AgentStore() == nil {
		return nil, huma.Error500InternalServerError("agent store is not available")
	}

	agent, err := e.store.AgentStore().RecordHeartbeat(ctx, store.AgentHeartbeatParams{
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
}

func (e *agentEndpoint) tasks(ctx context.Context, input *agentTasksInput) (*agentTasksOutput, error) {
	if e.store == nil || e.store.AgentStore() == nil || e.store.MonitorStore() == nil {
		return nil, huma.Error500InternalServerError("agent task stores are not available")
	}

	agent, err := e.store.AgentStore().Authenticate(ctx, input.AgentID, input.Token)
	if err != nil {
		return nil, apiError(err)
	}
	monitors, err := e.store.MonitorStore().ListAssignedEnabled(ctx, agent.ID)
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
}

func (e *agentEndpoint) syncMonitors(ctx context.Context, input *agentMonitorSyncInput) (*agentMonitorSyncOutput, error) {
	if e.store == nil || e.store.AgentStore() == nil || e.store.MonitorStore() == nil {
		return nil, huma.Error500InternalServerError("agent monitor stores are not available")
	}

	agent, err := e.store.AgentStore().Authenticate(ctx, input.Body.AgentID, input.Body.Token)
	if err != nil {
		return nil, apiError(err)
	}

	synced, err := e.syncAgentMonitors(ctx, agent, input.Body.Monitors)
	if err != nil {
		return nil, apiError(err)
	}

	out := &agentMonitorSyncOutput{}
	out.Body.Synced = synced
	return out, nil
}

func (e *agentEndpoint) syncAgentMonitors(ctx context.Context, agent model.Agent, monitors []protocol.AgentDiscoveredMonitor) (int, error) {
	synced := 0
	for index := range monitors {
		if err := e.syncAgentMonitor(ctx, agent, monitors[index]); err != nil {
			return 0, err
		}
		synced++
	}
	return synced, nil
}

func (e *agentEndpoint) syncAgentMonitor(ctx context.Context, agent model.Agent, discovered protocol.AgentDiscoveredMonitor) error {
	environmentID, err := e.store.EnvironmentIDForAgent(ctx, agent, discovered.EnvironmentCode)
	if err != nil {
		return fmt.Errorf("resolve discovered monitor environment: %w", err)
	}
	monitor, err := e.store.MonitorStore().UpsertDiscovered(ctx, store.UpsertDiscoveredMonitorParams{
		SourceKey:         discovered.SourceKey,
		Name:              discovered.Name,
		Type:              model.MonitorType(normalizeProtocolString(discovered.Type)),
		Target:            discovered.Target,
		GroupName:         discovered.GroupName,
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
	if err := e.store.MonitorStore().AssignAgent(ctx, monitor.ID, agent.ID); err != nil {
		return fmt.Errorf("assign discovered monitor: %w", err)
	}
	return nil
}

func (e *agentEndpoint) reportResult(ctx context.Context, input *agentResultsInput) (*statusOutput, error) {
	if e.store == nil || e.store.AgentStore() == nil || e.store.ResultStore() == nil {
		return nil, huma.Error500InternalServerError("agent result stores are not available")
	}

	agent, err := e.store.AgentStore().Authenticate(ctx, input.Body.AgentID, input.Body.Token)
	if err != nil {
		return nil, apiError(err)
	}
	params := store.RecordProbeResultParams{
		Agent:        agent,
		MonitorID:    input.Body.MonitorID,
		Status:       modelStatus(input.Body.Status),
		Latency:      time.Duration(input.Body.LatencyMS) * time.Millisecond,
		ErrorMessage: input.Body.ErrorMessage,
		CheckedAt:    input.Body.CheckedAt,
		RawDetail:    input.Body.RawDetail,
	}
	if err := e.recordProbeResult(ctx, params); err != nil {
		return nil, apiError(err)
	}

	return newStatusOutput("accepted"), nil
}

func (e *agentEndpoint) recordProbeResult(ctx context.Context, params store.RecordProbeResultParams) error {
	if e.resultIngestor != nil {
		if err := e.resultIngestor.Enqueue(ctx, params); err != nil {
			return fmt.Errorf("enqueue probe result: %w", err)
		}
		return nil
	}
	_, err := e.store.ResultStore().Record(ctx, params)
	if err != nil {
		return fmt.Errorf("record probe result: %w", err)
	}
	return nil
}
