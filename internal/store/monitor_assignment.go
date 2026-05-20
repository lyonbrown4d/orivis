package store

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/arcgolabs/dbx/querydsl"
	repository "github.com/arcgolabs/dbx/repository"
	"github.com/lyonbrown4d/orivis/internal/model"
)

func (s *monitorStore) assignMonitorIfUnassigned(ctx context.Context, monitorID string, agentIDs []string) error {
	_, err := s.repositories.monitorAgents.FirstSpec(ctx, repository.Where(monitorAgentsSchema.MonitorID.Eq(monitorID)))
	if err == nil {
		return nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return fmt.Errorf("find monitor owner: %w", err)
	}

	agentID, err := monitorAssignedAgent(monitorID, agentIDs)
	if err != nil {
		return fmt.Errorf("pick monitor owner: %w", err)
	}
	return s.assignMonitorOwner(ctx, monitorID, agentID)
}

func (s *monitorStore) normalizeMonitorIDs(monitorIDs []string) []string {
	normalized := make([]string, 0, len(monitorIDs))
	seen := make(map[string]struct{}, len(monitorIDs))
	for _, monitorID := range monitorIDs {
		monitorID = strings.TrimSpace(monitorID)
		if monitorID == "" {
			continue
		}
		if _, ok := seen[monitorID]; ok {
			continue
		}
		seen[monitorID] = struct{}{}
		normalized = append(normalized, monitorID)
	}
	return normalized
}

func (s *monitorStore) listAgentIDsForMonitorAssignment(ctx context.Context) ([]string, error) {
	rows, err := s.repositories.agents.List(
		ctx,
		querydsl.Select(querydsl.AllColumns(agentsSchema).Values()...).
			From(agentsSchema).
			OrderBy(agentsSchema.ID.Asc()),
	)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}

	ids := make([]string, 0, rows.Len())
	values := rows.Values()
	for i := range values {
		row := values[i]
		if strings.EqualFold(strings.TrimSpace(row.Status), string(model.AgentStatusDisabled)) {
			continue
		}
		id := strings.TrimSpace(row.ID)
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func monitorAssignedAgent(monitorID string, agentIDs []string) (string, error) {
	if len(agentIDs) == 0 {
		return "", fmt.Errorf("%w: no available agents", ErrNotFound)
	}

	h := fnv.New32a()
	if _, err := h.Write([]byte(monitorID)); err != nil {
		return "", fmt.Errorf("hash monitor id: %w", err)
	}
	return agentIDs[int(h.Sum32())%len(agentIDs)], nil
}
