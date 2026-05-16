package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/hashicorp/go-memdb"
	"github.com/lyonbrown4d/orivis/internal/model"
)

func (s *memoryAgentStore) Register(ctx context.Context, params RegisterAgentParams) (model.Agent, error) {
	_ = ctx

	normalized, err := normalizeRegisterParams(params)
	if err != nil {
		return model.Agent{}, err
	}

	now := time.Now().UTC()
	txn := s.store.db.Txn(true)
	defer txn.Abort()

	regionID, environmentIDs, err := ensureMemoryAgentScope(txn, normalized, now)
	if err != nil {
		return model.Agent{}, err
	}
	existing, found, err := memoryAgentByName(txn, normalized.Name)
	if err != nil {
		return model.Agent{}, err
	}
	if found {
		return updateMemoryAgentRegistration(txn, existing, normalized, regionID, environmentIDs, now)
	}
	return createMemoryAgentRegistration(txn, normalized, regionID, environmentIDs, now)
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

	existing, err := memoryAgentByID(txn, agentID)
	if err != nil {
		return model.Agent{}, err
	}
	if existing.Status == model.AgentStatusDisabled {
		return model.Agent{}, fmt.Errorf("%w: agent is disabled", ErrUnauthorized)
	}
	if authErr := verifyAgentToken(existing.TokenHash, params.Token); authErr != nil {
		return model.Agent{}, authErr
	}

	agent := cloneAgent(existing)
	agent.Version = strings.TrimSpace(params.Version)
	agent.LastSeenAt = seenAt
	agent.Status = model.AgentStatusOnline
	agent.UpdatedAt = time.Now().UTC()

	if replaceErr := replaceMemoryAgent(txn, existing, agent); replaceErr != nil {
		return model.Agent{}, replaceErr
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

	agent, err := memoryAgentByID(txn, id)
	if err != nil {
		return model.Agent{}, err
	}
	environmentIDs, err := memoryAgentEnvironmentIDs(txn, id)
	if err != nil {
		return model.Agent{}, err
	}
	out := cloneAgent(agent)
	out.EnvironmentIDs = collectionlist.NewList[string](environmentIDs...)
	return out, nil
}

func ensureMemoryAgentScope(txn *memdb.Txn, normalized normalizedRegisterParams, now time.Time) (string, []string, error) {
	regionID, err := ensureMemoryRegion(txn, normalized.RegionCode, now)
	if err != nil {
		return "", nil, err
	}
	environmentIDs, err := ensureMemoryEnvironments(txn, normalized.EnvironmentCodes, now)
	if err != nil {
		return "", nil, err
	}
	return regionID, environmentIDs, nil
}

func memoryAgentByName(txn *memdb.Txn, name string) (*model.Agent, bool, error) {
	raw, err := txn.First(memoryTableAgents, "name", name)
	if err != nil {
		return nil, false, fmt.Errorf("find memory agent by name: %w", err)
	}
	if raw == nil {
		return nil, false, nil
	}
	agent, err := memoryValue[model.Agent](raw, "agent")
	if err != nil {
		return nil, false, err
	}
	return agent, true, nil
}

func memoryAgentByID(txn *memdb.Txn, id string) (*model.Agent, error) {
	raw, err := txn.First(memoryTableAgents, "id", id)
	if err != nil {
		return nil, fmt.Errorf("get memory agent: %w", err)
	}
	if raw == nil {
		return nil, fmt.Errorf("%w: agent %s", ErrNotFound, id)
	}
	return memoryValue[model.Agent](raw, "agent")
}

func updateMemoryAgentRegistration(
	txn *memdb.Txn,
	existing *model.Agent,
	normalized normalizedRegisterParams,
	regionID string,
	environmentIDs []string,
	now time.Time,
) (model.Agent, error) {
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

func createMemoryAgentRegistration(
	txn *memdb.Txn,
	normalized normalizedRegisterParams,
	regionID string,
	environmentIDs []string,
	now time.Time,
) (model.Agent, error) {
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
