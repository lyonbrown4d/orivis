package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/dbx/querydsl"
	repository "github.com/arcgolabs/dbx/repository"
	"github.com/lyonbrown4d/orivis/internal/model"
)

func (s *agentStore) findAgentCredentialByName(ctx context.Context, name string) (agentCredential, error) {
	agent, err := s.repositories.agents.FirstSpec(ctx, repository.Where(agentsSchema.Name.Eq(name)))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return agentCredential{}, nil
		}
		return agentCredential{}, fmt.Errorf("find agent by name: %w", err)
	}
	return agentCredential{
		ID:        agent.ID,
		TokenHash: agent.TokenHash,
		Found:     true,
	}, nil
}

func (s *agentStore) updateRegisteredAgent(
	ctx context.Context,
	credential agentCredential,
	normalized normalizedRegisterParams,
	regionID string,
	environmentIDs []string,
	now time.Time,
) (model.Agent, error) {
	if err := verifyAgentToken(credential.TokenHash, normalized.Token); err != nil {
		return model.Agent{}, err
	}
	if err := s.updateExistingAgent(ctx, credential.ID, regionID, normalized.RuntimeType, normalized.Version, now); err != nil {
		return model.Agent{}, err
	}
	if err := s.replaceAgentEnvironments(ctx, credential.ID, environmentIDs); err != nil {
		return model.Agent{}, err
	}
	return s.Get(ctx, credential.ID)
}

func (s *agentStore) createRegisteredAgent(
	ctx context.Context,
	normalized normalizedRegisterParams,
	regionID string,
	environmentIDs []string,
	now time.Time,
) (model.Agent, error) {
	id, err := s.ids.NewID(ctx, "agt")
	if err != nil {
		return model.Agent{}, fmt.Errorf("generate agent id: %w", err)
	}
	tokenHash, err := hashAgentToken(normalized.Token)
	if err != nil {
		return model.Agent{}, err
	}
	if err := s.insertAgent(ctx, id, tokenHash, regionID, normalized, now); err != nil {
		if isCodeEntityConflict(err) {
			return s.updateAgentAfterCreateConflict(ctx, normalized, regionID, environmentIDs, now)
		}
		return model.Agent{}, err
	}
	if err := s.replaceAgentEnvironments(ctx, id, environmentIDs); err != nil {
		return model.Agent{}, err
	}
	return s.Get(ctx, id)
}

func (s *agentStore) updateAgentAfterCreateConflict(
	ctx context.Context,
	normalized normalizedRegisterParams,
	regionID string,
	environmentIDs []string,
	now time.Time,
) (model.Agent, error) {
	credential, err := s.findAgentCredentialByName(ctx, normalized.Name)
	if err != nil {
		return model.Agent{}, fmt.Errorf("resolve agent create conflict: %w", err)
	}
	if !credential.Found {
		return model.Agent{}, fmt.Errorf("resolve agent create conflict: %w", repository.ErrNotFound)
	}
	return s.updateRegisteredAgent(ctx, credential, normalized, regionID, environmentIDs, now)
}

func (s *agentStore) insertAgent(
	ctx context.Context,
	id string,
	tokenHash string,
	regionID string,
	normalized normalizedRegisterParams,
	now time.Time,
) error {
	agent := agentRecord{
		ID:          id,
		Name:        normalized.Name,
		TokenHash:   tokenHash,
		RegionID:    regionID,
		RuntimeType: normalized.RuntimeType,
		Version:     normalized.Version,
		LastSeenAt:  formatTime(now),
		Status:      string(model.AgentStatusOnline),
		Source:      string(model.ConfigSourceAPI),
		CreatedAt:   formatTime(now),
		UpdatedAt:   formatTime(now),
	}
	if err := s.repositories.agents.Create(ctx, &agent); err != nil {
		return fmt.Errorf("insert agent: %w", err)
	}
	return nil
}

func (s *agentStore) getAgentRecord(ctx context.Context, id string) (agentRecord, error) {
	rec, err := s.repositories.agents.FirstSpec(ctx, repository.Where(agentsSchema.ID.Eq(id)))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return agentRecord{}, fmt.Errorf("%w: agent %s", ErrNotFound, id)
		}
		return agentRecord{}, fmt.Errorf("get agent: %w", err)
	}
	return rec, nil
}

func (s *agentStore) ensureRegion(ctx context.Context, code string, now time.Time) (string, error) {
	return ensureCodeEntity(ctx, code, "reg", "region", s.ids, s.findRegionIDByCode, func(ctx context.Context, id, code string) error {
		return s.insertRegion(ctx, id, code, now)
	})
}

func (s *agentStore) findRegionIDByCode(ctx context.Context, code string) (string, error) {
	region, err := s.repositories.regions.FirstSpec(ctx, repository.Where(regionsSchema.Code.Eq(code)))
	if err != nil {
		return "", fmt.Errorf("find region by code: %w", err)
	}
	return region.ID, nil
}

func (s *agentStore) insertRegion(ctx context.Context, id, code string, now time.Time) error {
	row := regionRow{
		ID:        id,
		Name:      code,
		Code:      code,
		Enabled:   1,
		Source:    string(model.ConfigSourceAPI),
		CreatedAt: formatTime(now),
		UpdatedAt: formatTime(now),
	}
	if err := s.repositories.regions.Create(ctx, &row); err != nil {
		return fmt.Errorf("insert region: %w", err)
	}
	return nil
}

func (s *agentStore) ensureEnvironments(ctx context.Context, codes []string, now time.Time) ([]string, error) {
	ids, err := collectionlist.ReduceErrList(
		collectionlist.NewList(codes...),
		collectionlist.NewListWithCapacity[string](len(codes)),
		func(out *collectionlist.List[string], _ int, code string) (*collectionlist.List[string], error) {
			code = normalizeCode(code)
			if code == "" {
				return out, nil
			}
			id, err := s.ensureEnvironment(ctx, code, now)
			if err != nil {
				return nil, err
			}
			out.Add(id)
			return out, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("ensure agent environments: %w", err)
	}
	return ids.Values(), nil
}

func (s *agentStore) ensureEnvironment(ctx context.Context, code string, now time.Time) (string, error) {
	return ensureCodeEntity(ctx, code, "env", "environment", s.ids, s.findEnvironmentIDByCode, func(ctx context.Context, id, code string) error {
		return s.insertEnvironment(ctx, id, code, now)
	})
}

func (s *agentStore) findEnvironmentIDByCode(ctx context.Context, code string) (string, error) {
	environment, err := s.repositories.environments.FirstSpec(ctx, repository.Where(environmentsSchema.Code.Eq(code)))
	if err != nil {
		return "", fmt.Errorf("find environment by code: %w", err)
	}
	return environment.ID, nil
}

func (s *agentStore) insertEnvironment(ctx context.Context, id, code string, now time.Time) error {
	row := environmentRow{
		ID:        id,
		Name:      code,
		Code:      code,
		Enabled:   1,
		Source:    string(model.ConfigSourceAPI),
		CreatedAt: formatTime(now),
		UpdatedAt: formatTime(now),
	}
	if err := s.repositories.environments.Create(ctx, &row); err != nil {
		return fmt.Errorf("insert environment: %w", err)
	}
	return nil
}

func (s *agentStore) updateExistingAgent(ctx context.Context, id, regionID, runtimeType, version string, now time.Time) error {
	schema := agentsSchema
	_, err := s.repositories.agents.Update(
		ctx,
		querydsl.Update(schema).
			Set(
				schema.RegionID.Set(regionID),
				schema.RuntimeType.Set(runtimeType),
				schema.Version.Set(version),
				schema.LastSeenAt.Set(formatTime(now)),
				schema.Status.Set(string(model.AgentStatusOnline)),
				schema.UpdatedAt.Set(formatTime(now)),
			).
			Where(schema.ID.Eq(id)),
	)
	if err != nil {
		return fmt.Errorf("update existing agent: %w", err)
	}
	return nil
}

func (s *agentStore) replaceAgentEnvironments(ctx context.Context, agentID string, environmentIDs []string) error {
	repo := s.repositories.agentEnvironments
	schema := agentEnvironmentsSchema
	if _, err := repo.Delete(ctx, querydsl.DeleteFrom(schema).Where(schema.AgentID.Eq(agentID))); err != nil {
		return fmt.Errorf("clear agent environments: %w", err)
	}
	if len(environmentIDs) == 0 {
		return nil
	}
	rows := collectionlist.MapList(
		collectionlist.NewList(environmentIDs...),
		func(_ int, environmentID string) *agentEnvironmentRow {
			return &agentEnvironmentRow{
				AgentID:       agentID,
				EnvironmentID: environmentID,
			}
		},
	).Values()
	if err := repo.CreateMany(ctx, rows...); err != nil {
		return fmt.Errorf("insert agent environments: %w", err)
	}
	return nil
}

func (s *agentStore) agentEnvironmentIDs(ctx context.Context, agentID string) (*collectionlist.List[string], error) {
	schema := agentEnvironmentsSchema
	links, err := s.repositories.agentEnvironments.List(
		ctx,
		querydsl.Select(querydsl.AllColumns(schema).Values()...).
			From(schema).
			Where(schema.AgentID.Eq(agentID)).
			OrderBy(schema.EnvironmentID.Asc()),
	)
	if err != nil {
		return nil, fmt.Errorf("query agent environments: %w", err)
	}
	return collectionlist.MapList(links, func(_ int, link agentEnvironmentRow) string {
		return link.EnvironmentID
	}), nil
}
