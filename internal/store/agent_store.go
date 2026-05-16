package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/arcgolabs/dbx/querydsl"
	"github.com/lyonbrown4d/orivis/internal/model"
)

type AgentStore interface {
	Register(ctx context.Context, params RegisterAgentParams) (model.Agent, error)
	RecordHeartbeat(ctx context.Context, params AgentHeartbeatParams) (model.Agent, error)
	Authenticate(ctx context.Context, agentID, token string) (model.Agent, error)
	Get(ctx context.Context, id string) (model.Agent, error)
}

type agentStore struct {
	repositories *Repositories
	ids          IDGenerator
}

func (s *agentStore) Register(ctx context.Context, params RegisterAgentParams) (model.Agent, error) {
	normalized, err := normalizeRegisterParams(params)
	if err != nil {
		return model.Agent{}, err
	}

	now := time.Now().UTC()
	regionID, err := s.ensureRegion(ctx, normalized.RegionCode, now)
	if err != nil {
		return model.Agent{}, err
	}
	environmentIDs, err := s.ensureEnvironments(ctx, normalized.EnvironmentCodes, now)
	if err != nil {
		return model.Agent{}, err
	}

	credential, err := s.findAgentCredentialByName(ctx, normalized.Name)
	if err != nil {
		return model.Agent{}, err
	}
	if credential.Found {
		return s.updateRegisteredAgent(ctx, credential, normalized, regionID, environmentIDs, now)
	}
	return s.createRegisteredAgent(ctx, normalized, regionID, environmentIDs, now)
}

func (s *agentStore) RecordHeartbeat(ctx context.Context, params AgentHeartbeatParams) (model.Agent, error) {
	agentID := strings.TrimSpace(params.AgentID)
	if agentID == "" {
		return model.Agent{}, fmt.Errorf("%w: agent id is required", ErrInvalidInput)
	}

	seenAt := params.SeenAt.UTC()
	if seenAt.IsZero() {
		seenAt = time.Now().UTC()
	}

	if _, err := s.Authenticate(ctx, agentID, params.Token); err != nil {
		return model.Agent{}, err
	}

	schema := agentsSchema
	if _, err := s.repositories.agents.Update(
		ctx,
		querydsl.Update(schema).
			Set(
				schema.Version.Set(strings.TrimSpace(params.Version)),
				schema.LastSeenAt.Set(formatTime(seenAt)),
				schema.Status.Set(string(model.AgentStatusOnline)),
				schema.UpdatedAt.Set(formatTime(time.Now().UTC())),
			).
			Where(schema.ID.Eq(agentID)),
	); err != nil {
		return model.Agent{}, fmt.Errorf("record heartbeat: %w", err)
	}

	return s.Get(ctx, agentID)
}

func (s *agentStore) Authenticate(ctx context.Context, agentID, token string) (model.Agent, error) {
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

func (s *agentStore) Get(ctx context.Context, id string) (model.Agent, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return model.Agent{}, fmt.Errorf("%w: agent id is required", ErrInvalidInput)
	}

	rec, err := s.getAgentRecord(ctx, id)
	if err != nil {
		return model.Agent{}, err
	}
	environmentIDs, err := s.agentEnvironmentIDs(ctx, id)
	if err != nil {
		return model.Agent{}, err
	}
	return rec.model(environmentIDs)
}
