package store

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/lyonbrown4d/orivis/internal/model"
)

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
	result := memoryProbeResult(id, normalized, monitor, time.Now().UTC())
	if err := txn.Insert(memoryTableProbeResults, &result); err != nil {
		return model.ProbeResult{}, fmt.Errorf("insert memory probe result: %w", err)
	}
	txn.Commit()
	return cloneProbeResult(&result), nil
}

func memoryProbeResult(id string, normalized normalizedProbeResultParams, monitor model.Monitor, now time.Time) model.ProbeResult {
	return model.ProbeResult{
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
}

func memoryMonitorForAgent(txn *memdb.Txn, monitorID, agentID string) (model.Monitor, error) {
	rawLink, err := txn.First(memoryTableMonitorAgents, "id", memoryJoinID(monitorID, agentID))
	if err != nil {
		return model.Monitor{}, fmt.Errorf("find memory monitor assignment: %w", err)
	}
	if rawLink == nil {
		return model.Monitor{}, fmt.Errorf("%w: assigned monitor %s", ErrNotFound, monitorID)
	}

	monitor, err := memoryMonitorByID(txn, monitorID)
	if err != nil {
		return model.Monitor{}, err
	}
	if !monitor.Enabled {
		return model.Monitor{}, fmt.Errorf("%w: assigned monitor %s", ErrNotFound, monitorID)
	}
	return cloneMonitor(monitor), nil
}
