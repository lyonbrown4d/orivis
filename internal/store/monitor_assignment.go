package store

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"

	collectionlist "github.com/arcgolabs/collectionx/list"
	collectionset "github.com/arcgolabs/collectionx/set"
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
	seen := collectionset.NewSetWithCapacity[string](len(monitorIDs))
	return collectionlist.FilterMapList(collectionlist.NewList(monitorIDs...), func(_ int, monitorID string) (string, bool) {
		monitorID = strings.TrimSpace(monitorID)
		if monitorID == "" {
			return "", false
		}
		if seen.Contains(monitorID) {
			return "", false
		}
		seen.Add(monitorID)
		return monitorID, true
	}).Values()
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

	return collectionlist.FilterMapList(rows, func(_ int, row agentRecord) (string, bool) {
		if strings.EqualFold(strings.TrimSpace(row.Status), string(model.AgentStatusDisabled)) {
			return "", false
		}
		id := strings.TrimSpace(row.ID)
		return id, id != ""
	}).Values(), nil
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
