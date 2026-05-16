package store

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/lyonbrown4d/orivis/internal/model"
)

func (s *memoryMonitorStore) Create(ctx context.Context, params CreateMonitorParams) (model.Monitor, error) {
	_ = ctx

	normalized, err := normalizeCreateMonitorParams(params)
	if err != nil {
		return model.Monitor{}, err
	}

	txn := s.store.db.Txn(true)
	defer txn.Abort()

	if ensureErr := ensureMemoryEnvironmentExists(txn, normalized.EnvironmentID); ensureErr != nil {
		return model.Monitor{}, ensureErr
	}
	return createMemoryMonitor(txn, normalized, time.Now().UTC(), "insert memory monitor")
}

func (s *memoryMonitorStore) UpsertDiscovered(ctx context.Context, params UpsertDiscoveredMonitorParams) (model.Monitor, error) {
	_ = ctx

	normalized, err := normalizeDiscoveredMonitorParams(params)
	if err != nil {
		return model.Monitor{}, err
	}

	txn := s.store.db.Txn(true)
	defer txn.Abort()

	if ensureErr := ensureMemoryEnvironmentExists(txn, normalized.EnvironmentID); ensureErr != nil {
		return model.Monitor{}, ensureErr
	}
	existing, found, err := memoryMonitorBySourceKey(txn, normalized.SourceKey)
	if err != nil {
		return model.Monitor{}, err
	}
	if found {
		return updateMemoryDiscoveredMonitor(txn, existing, normalized, time.Now().UTC())
	}
	return createMemoryMonitor(txn, normalized, time.Now().UTC(), "insert memory discovered monitor")
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

	if err := ensureMemoryMonitorAndAgent(txn, monitorID, agentID); err != nil {
		return err
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

	monitors, err := memoryAssignedEnabledMonitors(txn, agentID)
	if err != nil {
		return nil, err
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

	monitor, err := memoryMonitorByID(txn, id)
	if err != nil {
		return model.Monitor{}, err
	}
	return cloneMonitor(monitor), nil
}

func ensureMemoryEnvironmentExists(txn *memdb.Txn, environmentID string) error {
	rawEnvironment, err := txn.First(memoryTableEnvironments, "id", environmentID)
	if err != nil {
		return fmt.Errorf("find memory environment: %w", err)
	}
	if rawEnvironment == nil {
		return fmt.Errorf("%w: environment %s", ErrNotFound, environmentID)
	}
	return nil
}

func ensureMemoryMonitorAndAgent(txn *memdb.Txn, monitorID, agentID string) error {
	if _, err := memoryMonitorByID(txn, monitorID); err != nil {
		return err
	}
	if _, err := memoryAgentByID(txn, agentID); err != nil {
		return err
	}
	return nil
}

func memoryMonitorBySourceKey(txn *memdb.Txn, sourceKey string) (*model.Monitor, bool, error) {
	raw, err := txn.First(memoryTableMonitors, "source_key", sourceKey)
	if err != nil {
		return nil, false, fmt.Errorf("find memory discovered monitor: %w", err)
	}
	if raw == nil {
		return nil, false, nil
	}
	monitor, err := memoryValue[model.Monitor](raw, "monitor")
	if err != nil {
		return nil, false, err
	}
	return monitor, true, nil
}

func memoryMonitorByID(txn *memdb.Txn, id string) (*model.Monitor, error) {
	raw, err := txn.First(memoryTableMonitors, "id", id)
	if err != nil {
		return nil, fmt.Errorf("get memory monitor: %w", err)
	}
	if raw == nil {
		return nil, fmt.Errorf("%w: monitor %s", ErrNotFound, id)
	}
	return memoryValue[model.Monitor](raw, "monitor")
}

func createMemoryMonitor(txn *memdb.Txn, normalized createMonitorParams, now time.Time, message string) (model.Monitor, error) {
	id, err := newID("mon")
	if err != nil {
		return model.Monitor{}, err
	}
	monitor := memoryMonitorFromParams(id, normalized, now)
	if err := txn.Insert(memoryTableMonitors, &monitor); err != nil {
		return model.Monitor{}, fmt.Errorf("%s: %w", message, err)
	}
	txn.Commit()
	return cloneMonitor(&monitor), nil
}

func updateMemoryDiscoveredMonitor(
	txn *memdb.Txn,
	existing *model.Monitor,
	normalized createMonitorParams,
	now time.Time,
) (model.Monitor, error) {
	monitor := memoryMonitorFromParams(existing.ID, normalized, existing.CreatedAt)
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

func memoryMonitorFromParams(id string, params createMonitorParams, now time.Time) model.Monitor {
	return model.Monitor{
		ID:                id,
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
		Source:            params.Source,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func memoryAssignedEnabledMonitors(txn *memdb.Txn, agentID string) ([]model.Monitor, error) {
	it, err := txn.Get(memoryTableMonitorAgents, "agent", agentID)
	if err != nil {
		return nil, fmt.Errorf("list memory monitor assignments: %w", err)
	}

	monitors := make([]model.Monitor, 0)
	for raw := it.Next(); raw != nil; raw = it.Next() {
		monitor, ok, err := memoryAssignedEnabledMonitor(txn, raw)
		if err != nil {
			return nil, err
		}
		if ok {
			monitors = append(monitors, monitor)
		}
	}
	return monitors, nil
}

func memoryAssignedEnabledMonitor(txn *memdb.Txn, raw any) (model.Monitor, bool, error) {
	link, err := memoryValue[memoryMonitorAgent](raw, "monitor agent link")
	if err != nil {
		return model.Monitor{}, false, err
	}
	monitor, err := memoryMonitorByID(txn, link.MonitorID)
	if err != nil {
		return model.Monitor{}, false, err
	}
	if !monitor.Enabled {
		return model.Monitor{}, false, nil
	}
	return cloneMonitor(monitor), true, nil
}
