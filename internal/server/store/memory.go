package store

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/go-co-op/gocron"
	"github.com/hashicorp/go-memdb"
	"github.com/lyonbrown4d/orivis/internal/shared/model"
)

const (
	memoryTableAgents            = "agents"
	memoryTableRegions           = "regions"
	memoryTableEnvironments      = "environments"
	memoryTableMonitors          = "monitors"
	memoryTableAgentEnvironments = "agent_environments"
	memoryTableMonitorAgents     = "monitor_agents"
	memoryTableProbeResults      = "probe_results"

	memoryResultRetention = 24 * time.Hour
	memoryCleanupInterval = time.Minute
)

type memoryStore struct {
	db              *memdb.MemDB
	logger          *slog.Logger
	scheduler       *gocron.Scheduler
	resultRetention time.Duration
}

type memoryAgentStore struct {
	store *memoryStore
}

type memoryMonitorStore struct {
	store *memoryStore
}

type memoryResultStore struct {
	store *memoryStore
}

type memoryAgentEnvironment struct {
	ID            string
	AgentID       string
	EnvironmentID string
}

type memoryMonitorAgent struct {
	ID        string
	MonitorID string
	AgentID   string
}

func openMemoryStore(resultRetentionValue, cleanupIntervalValue string, logger *slog.Logger) (*Store, error) {
	db, err := memdb.NewMemDB(memorySchema())
	if err != nil {
		return nil, fmt.Errorf("create memory store: %w", err)
	}
	resultRetention, err := memoryDuration(resultRetentionValue, memoryResultRetention, "db.memory.result_retention")
	if err != nil {
		return nil, err
	}
	cleanupInterval, err := memoryDuration(cleanupIntervalValue, memoryCleanupInterval, "db.memory.cleanup_interval")
	if err != nil {
		return nil, err
	}

	scheduler := gocron.NewScheduler(time.UTC)
	memory := &memoryStore{
		db:              db,
		logger:          logger,
		scheduler:       scheduler,
		resultRetention: resultRetention,
	}
	if _, err := scheduler.Every(cleanupInterval).Do(func() {
		if err := memory.CleanupExpiredResults(); err != nil && logger != nil {
			logger.Warn("cleanup expired memory results failed", slog.Any("error", err))
		}
	}); err != nil {
		return nil, fmt.Errorf("schedule memory cleanup: %w", err)
	}
	scheduler.StartAsync()

	return &Store{
		memory:   memory,
		agents:   &memoryAgentStore{store: memory},
		monitors: &memoryMonitorStore{store: memory},
		results:  &memoryResultStore{store: memory},
	}, nil
}

func (s *memoryStore) Close() error {
	if s == nil || s.scheduler == nil {
		return nil
	}
	s.scheduler.Stop()
	return nil
}

func memoryDuration(value string, fallback time.Duration, name string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%w: %s must be a duration: %w", ErrInvalidInput, name, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("%w: %s must be positive", ErrInvalidInput, name)
	}
	return duration, nil
}

func (s *memoryStore) CleanupExpiredResults() error {
	if s == nil || s.resultRetention <= 0 {
		return nil
	}

	cutoff := time.Now().UTC().Add(-s.resultRetention)
	txn := s.db.Txn(true)
	defer txn.Abort()

	it, err := txn.Get(memoryTableProbeResults, "id")
	if err != nil {
		return fmt.Errorf("list memory probe results: %w", err)
	}

	expired := make([]any, 0)
	for raw := it.Next(); raw != nil; raw = it.Next() {
		result := raw.(*model.ProbeResult)
		if result.CreatedAt.Before(cutoff) {
			expired = append(expired, raw)
		}
	}

	for _, raw := range expired {
		if err := txn.Delete(memoryTableProbeResults, raw); err != nil {
			return fmt.Errorf("delete expired memory probe result: %w", err)
		}
	}
	txn.Commit()
	return nil
}

func (s *memoryStore) memoryEnvironmentIDByCode(code string) (string, error) {
	txn := s.db.Txn(false)
	defer txn.Abort()

	raw, err := txn.First(memoryTableEnvironments, "code", normalizeCode(code))
	if err != nil {
		return "", fmt.Errorf("find memory environment by code: %w", err)
	}
	if raw == nil {
		return "", fmt.Errorf("%w: environment %s", ErrNotFound, code)
	}
	return raw.(*model.Environment).ID, nil
}

func (s *memoryAgentStore) Register(ctx context.Context, params RegisterAgentParams) (model.Agent, error) {
	_ = ctx

	normalized, err := normalizeRegisterParams(params)
	if err != nil {
		return model.Agent{}, err
	}

	now := time.Now().UTC()
	txn := s.store.db.Txn(true)
	defer txn.Abort()

	regionID, err := ensureMemoryRegion(txn, normalized.RegionCode, now)
	if err != nil {
		return model.Agent{}, err
	}
	environmentIDs, err := ensureMemoryEnvironments(txn, normalized.EnvironmentCodes, now)
	if err != nil {
		return model.Agent{}, err
	}

	raw, err := txn.First(memoryTableAgents, "name", normalized.Name)
	if err != nil {
		return model.Agent{}, fmt.Errorf("find memory agent by name: %w", err)
	}
	if raw != nil {
		existing := raw.(*model.Agent)
		if err := verifyAgentToken(existing.TokenHash, normalized.Token); err != nil {
			return model.Agent{}, err
		}

		agent := cloneAgent(existing)
		agent.RegionID = regionID
		agent.RuntimeType = normalized.RuntimeType
		agent.Version = normalized.Version
		agent.LastSeenAt = now
		agent.Status = model.AgentStatusOnline
		agent.UpdatedAt = now

		if err := replaceMemoryAgent(txn, existing, agent); err != nil {
			return model.Agent{}, err
		}
		if err := replaceMemoryAgentEnvironments(txn, agent.ID, environmentIDs); err != nil {
			return model.Agent{}, err
		}
		txn.Commit()
		agent.EnvironmentIDs = collectionlist.NewList[string](environmentIDs...)
		return agent, nil
	}

	id, err := newID("agt")
	if err != nil {
		return model.Agent{}, err
	}
	tokenHash, err := hashAgentToken(normalized.Token)
	if err != nil {
		return model.Agent{}, err
	}
	agent := model.Agent{
		ID:             id,
		Name:           normalized.Name,
		TokenHash:      tokenHash,
		RegionID:       regionID,
		EnvironmentIDs: collectionlist.NewList[string](environmentIDs...),
		RuntimeType:    normalized.RuntimeType,
		Version:        normalized.Version,
		LastSeenAt:     now,
		Status:         model.AgentStatusOnline,
		Source:         model.ConfigSourceAPI,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	inserted := cloneAgent(&agent)
	if err := txn.Insert(memoryTableAgents, &inserted); err != nil {
		return model.Agent{}, fmt.Errorf("insert memory agent: %w", err)
	}
	if err := replaceMemoryAgentEnvironments(txn, id, environmentIDs); err != nil {
		return model.Agent{}, err
	}
	txn.Commit()
	return agent, nil
}

func (s *memoryAgentStore) RecordHeartbeat(ctx context.Context, params AgentHeartbeatParams) (model.Agent, error) {
	_ = ctx

	agentID := strings.TrimSpace(params.AgentID)
	if agentID == "" {
		return model.Agent{}, fmt.Errorf("%w: agent id is required", ErrInvalidInput)
	}

	seenAt := params.SeenAt.UTC()
	if seenAt.IsZero() {
		seenAt = time.Now().UTC()
	}

	txn := s.store.db.Txn(true)
	defer txn.Abort()

	raw, err := txn.First(memoryTableAgents, "id", agentID)
	if err != nil {
		return model.Agent{}, fmt.Errorf("get memory agent: %w", err)
	}
	if raw == nil {
		return model.Agent{}, fmt.Errorf("%w: agent %s", ErrNotFound, agentID)
	}
	existing := raw.(*model.Agent)
	if existing.Status == model.AgentStatusDisabled {
		return model.Agent{}, fmt.Errorf("%w: agent is disabled", ErrUnauthorized)
	}
	if err := verifyAgentToken(existing.TokenHash, params.Token); err != nil {
		return model.Agent{}, err
	}

	agent := cloneAgent(existing)
	agent.Version = strings.TrimSpace(params.Version)
	agent.LastSeenAt = seenAt
	agent.Status = model.AgentStatusOnline
	agent.UpdatedAt = time.Now().UTC()

	if err := replaceMemoryAgent(txn, existing, agent); err != nil {
		return model.Agent{}, err
	}
	environmentIDs, err := memoryAgentEnvironmentIDs(txn, agent.ID)
	if err != nil {
		return model.Agent{}, err
	}
	txn.Commit()
	agent.EnvironmentIDs = collectionlist.NewList[string](environmentIDs...)
	return agent, nil
}

func (s *memoryAgentStore) Authenticate(ctx context.Context, agentID, token string) (model.Agent, error) {
	agent, err := s.Get(ctx, agentID)
	if err != nil {
		return model.Agent{}, err
	}
	if agent.Status == model.AgentStatusDisabled {
		return model.Agent{}, fmt.Errorf("%w: agent is disabled", ErrUnauthorized)
	}
	if err := verifyAgentToken(agent.TokenHash, token); err != nil {
		return model.Agent{}, err
	}
	return agent, nil
}

func (s *memoryAgentStore) Get(ctx context.Context, id string) (model.Agent, error) {
	_ = ctx

	id = strings.TrimSpace(id)
	if id == "" {
		return model.Agent{}, fmt.Errorf("%w: agent id is required", ErrInvalidInput)
	}

	txn := s.store.db.Txn(false)
	defer txn.Abort()

	raw, err := txn.First(memoryTableAgents, "id", id)
	if err != nil {
		return model.Agent{}, fmt.Errorf("get memory agent: %w", err)
	}
	if raw == nil {
		return model.Agent{}, fmt.Errorf("%w: agent %s", ErrNotFound, id)
	}
	agent := cloneAgent(raw.(*model.Agent))
	environmentIDs, err := memoryAgentEnvironmentIDs(txn, id)
	if err != nil {
		return model.Agent{}, err
	}
	agent.EnvironmentIDs = collectionlist.NewList[string](environmentIDs...)
	return agent, nil
}

func (s *memoryMonitorStore) Create(ctx context.Context, params CreateMonitorParams) (model.Monitor, error) {
	_ = ctx

	normalized, err := normalizeCreateMonitorParams(params)
	if err != nil {
		return model.Monitor{}, err
	}

	txn := s.store.db.Txn(true)
	defer txn.Abort()

	rawEnvironment, err := txn.First(memoryTableEnvironments, "id", normalized.EnvironmentID)
	if err != nil {
		return model.Monitor{}, fmt.Errorf("find memory environment: %w", err)
	}
	if rawEnvironment == nil {
		return model.Monitor{}, fmt.Errorf("%w: environment %s", ErrNotFound, normalized.EnvironmentID)
	}

	id, err := newID("mon")
	if err != nil {
		return model.Monitor{}, err
	}
	now := time.Now().UTC()
	monitor := model.Monitor{
		ID:                id,
		SourceKey:         normalized.SourceKey,
		Name:              normalized.Name,
		Type:              normalized.Type,
		Target:            normalized.Target,
		EnvironmentID:     normalized.EnvironmentID,
		Enabled:           normalized.Enabled,
		Interval:          normalized.Interval,
		Timeout:           normalized.Timeout,
		RetryCount:        normalized.RetryCount,
		AggregationPolicy: normalized.AggregationPolicy,
		Source:            normalized.Source,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := txn.Insert(memoryTableMonitors, &monitor); err != nil {
		return model.Monitor{}, fmt.Errorf("insert memory monitor: %w", err)
	}
	txn.Commit()
	return cloneMonitor(&monitor), nil
}

func (s *memoryMonitorStore) UpsertDiscovered(ctx context.Context, params UpsertDiscoveredMonitorParams) (model.Monitor, error) {
	_ = ctx

	createParams := CreateMonitorParams{
		SourceKey:         params.SourceKey,
		Name:              params.Name,
		Type:              params.Type,
		Target:            params.Target,
		EnvironmentID:     params.EnvironmentID,
		Enabled:           params.Enabled,
		Interval:          params.Interval,
		Timeout:           params.Timeout,
		RetryCount:        params.RetryCount,
		AggregationPolicy: params.AggregationPolicy,
		Source:            model.ConfigSourceAPI,
	}
	normalized, err := normalizeCreateMonitorParams(createParams)
	if err != nil {
		return model.Monitor{}, err
	}
	if normalized.SourceKey == "" {
		return model.Monitor{}, fmt.Errorf("%w: monitor source key is required", ErrInvalidInput)
	}

	txn := s.store.db.Txn(true)
	defer txn.Abort()

	rawEnvironment, err := txn.First(memoryTableEnvironments, "id", normalized.EnvironmentID)
	if err != nil {
		return model.Monitor{}, fmt.Errorf("find memory environment: %w", err)
	}
	if rawEnvironment == nil {
		return model.Monitor{}, fmt.Errorf("%w: environment %s", ErrNotFound, normalized.EnvironmentID)
	}

	raw, err := txn.First(memoryTableMonitors, "source_key", normalized.SourceKey)
	if err != nil {
		return model.Monitor{}, fmt.Errorf("find memory discovered monitor: %w", err)
	}

	now := time.Now().UTC()
	if raw != nil {
		existing := raw.(*model.Monitor)
		monitor := cloneMonitor(existing)
		monitor.Name = normalized.Name
		monitor.Type = normalized.Type
		monitor.Target = normalized.Target
		monitor.EnvironmentID = normalized.EnvironmentID
		monitor.Enabled = normalized.Enabled
		monitor.Interval = normalized.Interval
		monitor.Timeout = normalized.Timeout
		monitor.RetryCount = normalized.RetryCount
		monitor.AggregationPolicy = normalized.AggregationPolicy
		monitor.Source = normalized.Source
		monitor.UpdatedAt = now
		if err := txn.Delete(memoryTableMonitors, existing); err != nil {
			return model.Monitor{}, fmt.Errorf("replace memory discovered monitor: %w", err)
		}
		if err := txn.Insert(memoryTableMonitors, &monitor); err != nil {
			return model.Monitor{}, fmt.Errorf("insert replacement memory discovered monitor: %w", err)
		}
		txn.Commit()
		return monitor, nil
	}

	id, err := newID("mon")
	if err != nil {
		return model.Monitor{}, err
	}
	monitor := model.Monitor{
		ID:                id,
		SourceKey:         normalized.SourceKey,
		Name:              normalized.Name,
		Type:              normalized.Type,
		Target:            normalized.Target,
		EnvironmentID:     normalized.EnvironmentID,
		Enabled:           normalized.Enabled,
		Interval:          normalized.Interval,
		Timeout:           normalized.Timeout,
		RetryCount:        normalized.RetryCount,
		AggregationPolicy: normalized.AggregationPolicy,
		Source:            normalized.Source,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := txn.Insert(memoryTableMonitors, &monitor); err != nil {
		return model.Monitor{}, fmt.Errorf("insert memory discovered monitor: %w", err)
	}
	txn.Commit()
	return monitor, nil
}

func (s *memoryMonitorStore) AssignAgent(ctx context.Context, monitorID, agentID string) error {
	_ = ctx

	monitorID = strings.TrimSpace(monitorID)
	agentID = strings.TrimSpace(agentID)
	if monitorID == "" {
		return fmt.Errorf("%w: monitor id is required", ErrInvalidInput)
	}
	if agentID == "" {
		return fmt.Errorf("%w: agent id is required", ErrInvalidInput)
	}

	txn := s.store.db.Txn(true)
	defer txn.Abort()

	rawMonitor, err := txn.First(memoryTableMonitors, "id", monitorID)
	if err != nil {
		return fmt.Errorf("find memory monitor: %w", err)
	}
	if rawMonitor == nil {
		return fmt.Errorf("%w: monitor %s", ErrNotFound, monitorID)
	}
	rawAgent, err := txn.First(memoryTableAgents, "id", agentID)
	if err != nil {
		return fmt.Errorf("find memory agent: %w", err)
	}
	if rawAgent == nil {
		return fmt.Errorf("%w: agent %s", ErrNotFound, agentID)
	}

	link := memoryMonitorAgent{
		ID:        memoryJoinID(monitorID, agentID),
		MonitorID: monitorID,
		AgentID:   agentID,
	}
	if err := txn.Insert(memoryTableMonitorAgents, &link); err != nil {
		return fmt.Errorf("assign memory monitor agent: %w", err)
	}
	txn.Commit()
	return nil
}

func (s *memoryMonitorStore) ListAssignedEnabled(ctx context.Context, agentID string) ([]model.Monitor, error) {
	_ = ctx

	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, fmt.Errorf("%w: agent id is required", ErrInvalidInput)
	}

	txn := s.store.db.Txn(false)
	defer txn.Abort()

	it, err := txn.Get(memoryTableMonitorAgents, "agent", agentID)
	if err != nil {
		return nil, fmt.Errorf("list memory monitor assignments: %w", err)
	}

	monitors := make([]model.Monitor, 0)
	for raw := it.Next(); raw != nil; raw = it.Next() {
		link := raw.(*memoryMonitorAgent)
		rawMonitor, err := txn.First(memoryTableMonitors, "id", link.MonitorID)
		if err != nil {
			return nil, fmt.Errorf("get assigned memory monitor: %w", err)
		}
		if rawMonitor == nil {
			continue
		}
		monitor := rawMonitor.(*model.Monitor)
		if monitor.Enabled {
			monitors = append(monitors, cloneMonitor(monitor))
		}
	}
	sort.Slice(monitors, func(i, j int) bool {
		return monitors[i].Name < monitors[j].Name
	})
	return monitors, nil
}

func (s *memoryMonitorStore) Get(ctx context.Context, id string) (model.Monitor, error) {
	_ = ctx

	id = strings.TrimSpace(id)
	if id == "" {
		return model.Monitor{}, fmt.Errorf("%w: monitor id is required", ErrInvalidInput)
	}

	txn := s.store.db.Txn(false)
	defer txn.Abort()

	raw, err := txn.First(memoryTableMonitors, "id", id)
	if err != nil {
		return model.Monitor{}, fmt.Errorf("get memory monitor: %w", err)
	}
	if raw == nil {
		return model.Monitor{}, fmt.Errorf("%w: monitor %s", ErrNotFound, id)
	}
	return cloneMonitor(raw.(*model.Monitor)), nil
}

func (s *memoryResultStore) Record(ctx context.Context, params RecordProbeResultParams) (model.ProbeResult, error) {
	_ = ctx

	normalized, err := normalizeProbeResultParams(params)
	if err != nil {
		return model.ProbeResult{}, err
	}

	txn := s.store.db.Txn(true)
	defer txn.Abort()

	monitor, err := memoryMonitorForAgent(txn, normalized.MonitorID, normalized.Agent.ID)
	if err != nil {
		return model.ProbeResult{}, err
	}

	id, err := newID("res")
	if err != nil {
		return model.ProbeResult{}, err
	}
	now := time.Now().UTC()
	result := model.ProbeResult{
		ID:            id,
		MonitorID:     monitor.ID,
		AgentID:       normalized.Agent.ID,
		RegionID:      normalized.Agent.RegionID,
		EnvironmentID: monitor.EnvironmentID,
		Status:        normalized.Status,
		Latency:       normalized.Latency,
		ErrorMessage:  normalized.ErrorMessage,
		CheckedAt:     normalized.CheckedAt,
		RawDetail:     append([]byte(nil), normalized.RawDetail...),
		CreatedAt:     now,
	}
	if err := txn.Insert(memoryTableProbeResults, &result); err != nil {
		return model.ProbeResult{}, fmt.Errorf("insert memory probe result: %w", err)
	}
	txn.Commit()
	return cloneProbeResult(&result), nil
}

func ensureMemoryRegion(txn *memdb.Txn, code string, now time.Time) (string, error) {
	raw, err := txn.First(memoryTableRegions, "code", code)
	if err != nil {
		return "", fmt.Errorf("find memory region: %w", err)
	}
	if raw != nil {
		return raw.(*model.Region).ID, nil
	}

	id, err := newID("reg")
	if err != nil {
		return "", err
	}
	region := model.Region{
		ID:        id,
		Name:      code,
		Code:      code,
		Enabled:   true,
		Source:    model.ConfigSourceAPI,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := txn.Insert(memoryTableRegions, &region); err != nil {
		return "", fmt.Errorf("insert memory region: %w", err)
	}
	return id, nil
}

func ensureMemoryEnvironments(txn *memdb.Txn, codes []string, now time.Time) ([]string, error) {
	out := make([]string, 0, len(codes))
	for _, code := range codes {
		code = normalizeCode(code)
		if code == "" {
			continue
		}
		id, err := ensureMemoryEnvironment(txn, code, now)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

func ensureMemoryEnvironment(txn *memdb.Txn, code string, now time.Time) (string, error) {
	raw, err := txn.First(memoryTableEnvironments, "code", code)
	if err != nil {
		return "", fmt.Errorf("find memory environment: %w", err)
	}
	if raw != nil {
		return raw.(*model.Environment).ID, nil
	}

	id, err := newID("env")
	if err != nil {
		return "", err
	}
	environment := model.Environment{
		ID:        id,
		Name:      code,
		Code:      code,
		Enabled:   true,
		Source:    model.ConfigSourceAPI,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := txn.Insert(memoryTableEnvironments, &environment); err != nil {
		return "", fmt.Errorf("insert memory environment: %w", err)
	}
	return id, nil
}

func replaceMemoryAgent(txn *memdb.Txn, existing *model.Agent, next model.Agent) error {
	if err := txn.Delete(memoryTableAgents, existing); err != nil {
		return fmt.Errorf("replace memory agent: %w", err)
	}
	inserted := cloneAgent(&next)
	if err := txn.Insert(memoryTableAgents, &inserted); err != nil {
		return fmt.Errorf("insert replacement memory agent: %w", err)
	}
	return nil
}

func replaceMemoryAgentEnvironments(txn *memdb.Txn, agentID string, environmentIDs []string) error {
	it, err := txn.Get(memoryTableAgentEnvironments, "agent", agentID)
	if err != nil {
		return fmt.Errorf("list memory agent environments: %w", err)
	}
	existing := make([]any, 0)
	for raw := it.Next(); raw != nil; raw = it.Next() {
		existing = append(existing, raw)
	}
	for _, raw := range existing {
		if err := txn.Delete(memoryTableAgentEnvironments, raw); err != nil {
			return fmt.Errorf("clear memory agent environment: %w", err)
		}
	}
	for _, environmentID := range environmentIDs {
		link := memoryAgentEnvironment{
			ID:            memoryJoinID(agentID, environmentID),
			AgentID:       agentID,
			EnvironmentID: environmentID,
		}
		if err := txn.Insert(memoryTableAgentEnvironments, &link); err != nil {
			return fmt.Errorf("insert memory agent environment: %w", err)
		}
	}
	return nil
}

func memoryAgentEnvironmentIDs(txn *memdb.Txn, agentID string) ([]string, error) {
	it, err := txn.Get(memoryTableAgentEnvironments, "agent", agentID)
	if err != nil {
		return nil, fmt.Errorf("query memory agent environments: %w", err)
	}
	ids := make([]string, 0)
	for raw := it.Next(); raw != nil; raw = it.Next() {
		ids = append(ids, raw.(*memoryAgentEnvironment).EnvironmentID)
	}
	sort.Strings(ids)
	return ids, nil
}

func memoryMonitorForAgent(txn *memdb.Txn, monitorID, agentID string) (model.Monitor, error) {
	rawLink, err := txn.First(memoryTableMonitorAgents, "id", memoryJoinID(monitorID, agentID))
	if err != nil {
		return model.Monitor{}, fmt.Errorf("find memory monitor assignment: %w", err)
	}
	if rawLink == nil {
		return model.Monitor{}, fmt.Errorf("%w: assigned monitor %s", ErrNotFound, monitorID)
	}

	rawMonitor, err := txn.First(memoryTableMonitors, "id", monitorID)
	if err != nil {
		return model.Monitor{}, fmt.Errorf("find memory monitor for agent: %w", err)
	}
	if rawMonitor == nil {
		return model.Monitor{}, fmt.Errorf("%w: assigned monitor %s", ErrNotFound, monitorID)
	}
	monitor := rawMonitor.(*model.Monitor)
	if !monitor.Enabled {
		return model.Monitor{}, fmt.Errorf("%w: assigned monitor %s", ErrNotFound, monitorID)
	}
	return cloneMonitor(monitor), nil
}

func cloneAgent(agent *model.Agent) model.Agent {
	if agent == nil {
		return model.Agent{}
	}
	out := *agent
	if agent.EnvironmentIDs != nil {
		out.EnvironmentIDs = collectionlist.NewList[string](agent.EnvironmentIDs.Values()...)
	}
	return out
}

func cloneMonitor(monitor *model.Monitor) model.Monitor {
	if monitor == nil {
		return model.Monitor{}
	}
	return *monitor
}

func cloneProbeResult(result *model.ProbeResult) model.ProbeResult {
	if result == nil {
		return model.ProbeResult{}
	}
	out := *result
	out.RawDetail = append([]byte(nil), result.RawDetail...)
	return out
}

func memoryJoinID(parts ...string) string {
	return strings.Join(parts, "\x00")
}

func memorySchema() *memdb.DBSchema {
	return &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			memoryTableAgents: {
				Name: memoryTableAgents,
				Indexes: map[string]*memdb.IndexSchema{
					"id":   uniqueStringIndex("id", "ID"),
					"name": uniqueStringIndex("name", "Name"),
				},
			},
			memoryTableRegions: {
				Name: memoryTableRegions,
				Indexes: map[string]*memdb.IndexSchema{
					"id":   uniqueStringIndex("id", "ID"),
					"code": uniqueStringIndex("code", "Code"),
				},
			},
			memoryTableEnvironments: {
				Name: memoryTableEnvironments,
				Indexes: map[string]*memdb.IndexSchema{
					"id":   uniqueStringIndex("id", "ID"),
					"code": uniqueStringIndex("code", "Code"),
				},
			},
			memoryTableMonitors: {
				Name: memoryTableMonitors,
				Indexes: map[string]*memdb.IndexSchema{
					"id":         uniqueStringIndex("id", "ID"),
					"source_key": optionalStringIndex("source_key", "SourceKey"),
				},
			},
			memoryTableAgentEnvironments: {
				Name: memoryTableAgentEnvironments,
				Indexes: map[string]*memdb.IndexSchema{
					"id":    uniqueStringIndex("id", "ID"),
					"agent": stringIndex("agent", "AgentID"),
				},
			},
			memoryTableMonitorAgents: {
				Name: memoryTableMonitorAgents,
				Indexes: map[string]*memdb.IndexSchema{
					"id":      uniqueStringIndex("id", "ID"),
					"agent":   stringIndex("agent", "AgentID"),
					"monitor": stringIndex("monitor", "MonitorID"),
				},
			},
			memoryTableProbeResults: {
				Name: memoryTableProbeResults,
				Indexes: map[string]*memdb.IndexSchema{
					"id": uniqueStringIndex("id", "ID"),
				},
			},
		},
	}
}

func uniqueStringIndex(name, field string) *memdb.IndexSchema {
	return &memdb.IndexSchema{
		Name:    name,
		Unique:  true,
		Indexer: &memdb.StringFieldIndex{Field: field},
	}
}

func stringIndex(name, field string) *memdb.IndexSchema {
	return &memdb.IndexSchema{
		Name:    name,
		Indexer: &memdb.StringFieldIndex{Field: field},
	}
}

func optionalStringIndex(name, field string) *memdb.IndexSchema {
	return &memdb.IndexSchema{
		Name:         name,
		AllowMissing: true,
		Indexer:      &memdb.StringFieldIndex{Field: field},
	}
}
