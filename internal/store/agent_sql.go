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
	agent, err := newAgentRepository(s.db).FirstSpec(ctx, repository.Where(agentsSchema.Name.Eq(name)))
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
	id, err := newID("agt")
	if err != nil {
		return model.Agent{}, err
	}
	tokenHash, err := hashAgentToken(normalized.Token)
	if err != nil {
		return model.Agent{}, err
	}
	if err := s.insertAgent(ctx, id, tokenHash, regionID, normalized, now); err != nil {
		return model.Agent{}, err
	}
	if err := s.replaceAgentEnvironments(ctx, id, environmentIDs); err != nil {
		return model.Agent{}, err
	}
	return s.Get(ctx, id)
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
	if err := newAgentRepository(s.db).Create(ctx, &agent); err != nil {
		return fmt.Errorf("insert agent: %w", err)
	}
	return nil
}

func (s *agentStore) getAgentRecord(ctx context.Context, id string) (agentRecord, error) {
	rec, err := newAgentRepository(s.db).FirstSpec(ctx, repository.Where(agentsSchema.ID.Eq(id)))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return agentRecord{}, fmt.Errorf("%w: agent %s", ErrNotFound, id)
		}
		return agentRecord{}, fmt.Errorf("get agent: %w", err)
	}
	return rec, nil
}

func (s *agentStore) ensureRegion(ctx context.Context, code string, now time.Time) (string, error) {
	return ensureCodeEntity(ctx, code, "reg", "region", s.findRegionIDByCode, func(ctx context.Context, id, code string) error {
		return s.insertRegion(ctx, id, code, now)
	})
}

func (s *agentStore) findRegionIDByCode(ctx context.Context, code string) (string, error) {
	region, err := newRegionRepository(s.db).FirstSpec(ctx, repository.Where(regionsSchema.Code.Eq(code)))
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
	if err := newRegionRepository(s.db).Create(ctx, &row); err != nil {
		return fmt.Errorf("insert region: %w", err)
	}
	return nil
}

func (s *agentStore) ensureEnvironments(ctx context.Context, codes []string, now time.Time) ([]string, error) {
	out := make([]string, 0, len(codes))
	for _, code := range codes {
		code = normalizeCode(code)
		if code == "" {
			continue
		}
		id, err := s.ensureEnvironment(ctx, code, now)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

func (s *agentStore) ensureEnvironment(ctx context.Context, code string, now time.Time) (string, error) {
	return ensureCodeEntity(ctx, code, "env", "environment", s.findEnvironmentIDByCode, func(ctx context.Context, id, code string) error {
		return s.insertEnvironment(ctx, id, code, now)
	})
}

func (s *agentStore) findEnvironmentIDByCode(ctx context.Context, code string) (string, error) {
	environment, err := newEnvironmentRepository(s.db).FirstSpec(ctx, repository.Where(environmentsSchema.Code.Eq(code)))
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
	if err := newEnvironmentRepository(s.db).Create(ctx, &row); err != nil {
		return fmt.Errorf("insert environment: %w", err)
	}
	return nil
}

func ensureCodeEntity(
	ctx context.Context,
	code,
	idPrefix,
	entityName string,
	find func(context.Context, string) (string, error),
	insert func(context.Context, string, string) error,
) (string, error) {
	id, err := find(ctx, code)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return "", fmt.Errorf("find %s: %w", entityName, err)
	}

	id, err = newID(idPrefix)
	if err != nil {
		return "", err
	}
	if err := insert(ctx, id, code); err != nil {
		return "", err
	}
	return id, nil
}

func (s *agentStore) updateExistingAgent(ctx context.Context, id, regionID, runtimeType, version string, now time.Time) error {
	schema := agentsSchema
	_, err := newAgentRepository(s.db).Update(
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
	repo := newAgentEnvironmentRepository(s.db)
	schema := agentEnvironmentsSchema
	if _, err := repo.Delete(ctx, querydsl.DeleteFrom(schema).Where(schema.AgentID.Eq(agentID))); err != nil {
		return fmt.Errorf("clear agent environments: %w", err)
	}
	if len(environmentIDs) == 0 {
		return nil
	}
	rows := make([]*agentEnvironmentRow, 0, len(environmentIDs))
	for _, environmentID := range environmentIDs {
		rows = append(rows, &agentEnvironmentRow{
			AgentID:       agentID,
			EnvironmentID: environmentID,
		})
	}
	if err := repo.CreateMany(ctx, rows...); err != nil {
		return fmt.Errorf("insert agent environments: %w", err)
	}
	return nil
}

func (s *agentStore) agentEnvironmentIDs(ctx context.Context, agentID string) (*collectionlist.List[string], error) {
	schema := agentEnvironmentsSchema
	links, err := newAgentEnvironmentRepository(s.db).List(
		ctx,
		querydsl.Select(querydsl.AllColumns(schema).Values()...).
			From(schema).
			Where(schema.AgentID.Eq(agentID)).
			OrderBy(schema.EnvironmentID.Asc()),
	)
	if err != nil {
		return nil, fmt.Errorf("query agent environments: %w", err)
	}
	ids := make([]string, 0, links.Len())
	for _, link := range links.Values() {
		ids = append(ids, link.EnvironmentID)
	}
	return collectionlist.NewList[string](ids...), nil
}
