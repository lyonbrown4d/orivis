package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/arcgolabs/dbx"
	"github.com/arcgolabs/dbx/querydsl"
	repository "github.com/arcgolabs/dbx/repository"
	"github.com/lyonbrown4d/orivis/internal/model"
)

type MonitorStore interface {
	Create(ctx context.Context, params CreateMonitorParams) (model.Monitor, error)
	UpsertDiscovered(ctx context.Context, params UpsertDiscoveredMonitorParams) (model.Monitor, error)
	AssignAgent(ctx context.Context, monitorID, agentID string) error
	AssignMonitors(ctx context.Context, monitorIDs []string) error
	ListAssignedEnabled(ctx context.Context, agentID string) ([]model.Monitor, error)
	Get(ctx context.Context, id string) (model.Monitor, error)
}

type monitorStore struct {
	repositories *Repositories
	ids          IDGenerator
}

func (s *monitorStore) Create(ctx context.Context, params CreateMonitorParams) (model.Monitor, error) {
	normalized, err := normalizeCreateMonitorParams(params)
	if err != nil {
		return model.Monitor{}, err
	}

	id, err := s.ids.NewID(ctx, "mon")
	if err != nil {
		return model.Monitor{}, fmt.Errorf("generate monitor id: %w", err)
	}
	if err := s.insertMonitor(ctx, id, normalized); err != nil {
		return model.Monitor{}, err
	}
	return s.Get(ctx, id)
}

func (s *monitorStore) UpsertDiscovered(ctx context.Context, params UpsertDiscoveredMonitorParams) (model.Monitor, error) {
	normalized, err := normalizeDiscoveredMonitorParams(params)
	if err != nil {
		return model.Monitor{}, err
	}

	var existingID string
	existing, err := s.repositories.monitors.FirstSpec(ctx, repository.Where(monitorsSchema.SourceKey.Eq(normalized.SourceKey)))
	switch {
	case err == nil:
		existingID = existing.ID
		if updateErr := s.updateDiscoveredMonitor(ctx, existingID, normalized); updateErr != nil {
			return model.Monitor{}, updateErr
		}
		return s.Get(ctx, existingID)
	case errors.Is(err, repository.ErrNotFound):
		return s.Create(ctx, createMonitorParamsToPublic(normalized))
	default:
		return model.Monitor{}, fmt.Errorf("find discovered monitor: %w", err)
	}
}

func (s *monitorStore) AssignAgent(ctx context.Context, monitorID, agentID string) error {
	monitorID = strings.TrimSpace(monitorID)
	agentID = strings.TrimSpace(agentID)
	if monitorID == "" {
		return fmt.Errorf("%w: monitor id is required", ErrInvalidInput)
	}
	if agentID == "" {
		return fmt.Errorf("%w: agent id is required", ErrInvalidInput)
	}

	row := monitorAgentRow{
		MonitorID: monitorID,
		AgentID:   agentID,
	}
	err := s.repositories.monitorAgents.Upsert(
		ctx,
		&row,
		"monitor_id",
		"agent_id",
	)
	if err != nil {
		return fmt.Errorf("assign monitor agent: %w", err)
	}
	return nil
}

func (s *monitorStore) AssignMonitors(ctx context.Context, monitorIDs []string) error {
	if len(monitorIDs) == 0 {
		return nil
	}

	agentIDs, err := s.listAgentIDsForMonitorAssignment(ctx)
	if err != nil {
		return fmt.Errorf("list assignment agents: %w", err)
	}
	if len(agentIDs) == 0 {
		return fmt.Errorf("%w: no available agents", ErrNotFound)
	}

	normalized := s.normalizeMonitorIDs(monitorIDs)
	for _, monitorID := range normalized {
		if err := s.assignMonitorIfUnassigned(ctx, monitorID, agentIDs); err != nil {
			return fmt.Errorf("assign monitor %s: %w", monitorID, err)
		}
	}

	return nil
}

func (s *monitorStore) ListAssignedEnabled(ctx context.Context, agentID string) ([]model.Monitor, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, fmt.Errorf("%w: agent id is required", ErrInvalidInput)
	}

	query := querydsl.Select(querydsl.AllColumns(monitorsSchema).Values()...).
		From(monitorsSchema).
		Join(monitorAgentsSchema).
		On(monitorAgentsSchema.MonitorID.EqColumn(monitorsSchema.ID)).
		Where(querydsl.And(
			monitorAgentsSchema.AgentID.Eq(agentID),
			monitorsSchema.Enabled.Eq(1),
		)).
		OrderBy(monitorsSchema.Name.Asc())
	records, err := s.repositories.monitors.List(
		ctx,
		query,
	)
	if err != nil {
		return nil, fmt.Errorf("list assigned monitors: %w", err)
	}

	monitors := make([]model.Monitor, 0, records.Len())
	values := records.Values()
	for index := range values {
		monitor, err := values[index].model()
		if err != nil {
			return nil, fmt.Errorf("map assigned monitor: %w", err)
		}
		monitors = append(monitors, monitor)
	}
	return monitors, nil
}

func (s *monitorStore) Get(ctx context.Context, id string) (model.Monitor, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return model.Monitor{}, fmt.Errorf("%w: monitor id is required", ErrInvalidInput)
	}

	monitor, err := s.getMonitor(ctx, id)
	if err != nil {
		return model.Monitor{}, err
	}
	return monitor, nil
}

func (s *monitorStore) insertMonitor(ctx context.Context, id string, normalized createMonitorParams) error {
	now := time.Now().UTC()
	row := monitorRecord{
		ID:                id,
		SourceKey:         normalized.SourceKey,
		Name:              normalized.Name,
		Type:              string(normalized.Type),
		Target:            normalized.Target,
		GroupName:         normalized.GroupName,
		EnvironmentID:     normalized.EnvironmentID,
		Enabled:           boolInt(normalized.Enabled),
		IntervalSeconds:   int(normalized.Interval.Seconds()),
		TimeoutSeconds:    int(normalized.Timeout.Seconds()),
		RetryCount:        normalized.RetryCount,
		AggregationPolicy: string(normalized.AggregationPolicy),
		Source:            string(normalized.Source),
		CreatedAt:         formatTime(now),
		UpdatedAt:         formatTime(now),
	}
	if err := s.repositories.monitors.Create(ctx, &row); err != nil {
		return fmt.Errorf("insert monitor: %w", err)
	}
	return nil
}

func (s *monitorStore) updateDiscoveredMonitor(ctx context.Context, id string, normalized createMonitorParams) error {
	schema := monitorsSchema
	_, err := s.repositories.monitors.Update(
		ctx,
		querydsl.Update(schema).
			Set(
				schema.Name.Set(normalized.Name),
				schema.Type.Set(string(normalized.Type)),
				schema.Target.Set(normalized.Target),
				schema.GroupName.Set(normalized.GroupName),
				schema.EnvironmentID.Set(normalized.EnvironmentID),
				schema.Enabled.Set(boolInt(normalized.Enabled)),
				schema.IntervalSeconds.Set(int(normalized.Interval.Seconds())),
				schema.TimeoutSeconds.Set(int(normalized.Timeout.Seconds())),
				schema.RetryCount.Set(normalized.RetryCount),
				schema.AggregationPolicy.Set(string(normalized.AggregationPolicy)),
				schema.Source.Set(string(normalized.Source)),
				schema.UpdatedAt.Set(formatTime(time.Now().UTC())),
			).
			Where(schema.ID.Eq(id)),
	)
	if err != nil {
		return fmt.Errorf("update discovered monitor: %w", err)
	}
	return nil
}

func (s *monitorStore) getMonitor(ctx context.Context, id string) (model.Monitor, error) {
	record, err := s.repositories.monitors.FirstSpec(ctx, repository.Where(monitorsSchema.ID.Eq(id)))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.Monitor{}, fmt.Errorf("%w: monitor %s", ErrNotFound, id)
		}
		return model.Monitor{}, fmt.Errorf("get monitor: %w", err)
	}
	return record.model()
}

func (s *monitorStore) assignMonitorOwner(ctx context.Context, monitorID, agentID string) error {
	schema := monitorAgentsSchema
	row := monitorAgentRow{
		MonitorID: monitorID,
		AgentID:   agentID,
	}

	err := s.repositories.monitorAgents.InTx(ctx, nil, func(_ *dbx.Tx, txRepo *repository.Base[monitorAgentRow, monitorAgentSchema]) error {
		if _, deleteErr := txRepo.Delete(ctx, querydsl.DeleteFrom(schema).Where(schema.MonitorID.Eq(monitorID))); deleteErr != nil {
			return fmt.Errorf("clear existing monitor owner: %w", deleteErr)
		}
		if createErr := txRepo.Create(ctx, &row); createErr != nil {
			return fmt.Errorf("set monitor owner: %w", createErr)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("assign monitor owner: %w", err)
	}
	return nil
}
