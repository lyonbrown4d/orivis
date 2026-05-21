package store

import (
	"context"
	"errors"
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
		return wrapError(err, "find monitor owner")
	}

	agentID, err := monitorAssignedAgent(monitorID, agentIDs)
	if err != nil {
		return wrapError(err, "pick monitor owner")
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
		return nil, wrapError(err, "list agents")
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
		return "", wrapError(ErrNotFound, "no available agents")
	}

	h := fnv.New32a()
	if _, err := h.Write([]byte(monitorID)); err != nil {
		return "", wrapError(err, "hash monitor id")
	}
	return agentIDs[int(h.Sum32())%len(agentIDs)], nil
}
