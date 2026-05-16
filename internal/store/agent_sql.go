package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/lyonbrown4d/orivis/internal/model"
)

func (s *agentStore) findAgentCredentialByName(ctx context.Context, name string) (agentCredential, error) {
	var credential agentCredential
	err := s.db.QueryRowContext(ctx, "SELECT id, token_hash FROM agents WHERE name = ?", name).Scan(&credential.ID, &credential.TokenHash)
	if err == nil {
		credential.Found = true
		return credential, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return credential, nil
	}
	return credential, fmt.Errorf("find agent by name: %w", err)
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
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO agents (
                id, name, token_hash, region_id, runtime_type, version,
                last_seen_at, status, source, created_at, updated_at
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id,
		normalized.Name,
		tokenHash,
		regionID,
		normalized.RuntimeType,
		normalized.Version,
		formatTime(now),
		string(model.AgentStatusOnline),
		string(model.ConfigSourceAPI),
		formatTime(now),
		formatTime(now),
	)
	if err != nil {
		return fmt.Errorf("insert agent: %w", err)
	}
	return nil
}

func (s *agentStore) getAgentRecord(ctx context.Context, id string) (agentRecord, error) {
	var rec agentRecord
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, name, token_hash, region_id, runtime_type, version,
                last_seen_at, status, source, created_at, updated_at
         FROM agents
         WHERE id = ?`,
		id,
	).Scan(
		&rec.ID,
		&rec.Name,
		&rec.TokenHash,
		&rec.RegionID,
		&rec.RuntimeType,
		&rec.Version,
		&rec.LastSeenAt,
		&rec.Status,
		&rec.Source,
		&rec.CreatedAt,
		&rec.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return agentRecord{}, fmt.Errorf("%w: agent %s", ErrNotFound, id)
		}
		return agentRecord{}, fmt.Errorf("get agent: %w", err)
	}
	return rec, nil
}

func (s *agentStore) ensureRegion(ctx context.Context, code string, now time.Time) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, "SELECT id FROM regions WHERE code = ?", code).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("find region: %w", err)
	}

	id, err = newID("reg")
	if err != nil {
		return "", err
	}
	if _, err := s.db.ExecContext(
		ctx,
		`INSERT INTO regions (id, name, code, enabled, source, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id,
		code,
		code,
		1,
		string(model.ConfigSourceAPI),
		formatTime(now),
		formatTime(now),
	); err != nil {
		return "", fmt.Errorf("insert region: %w", err)
	}
	return id, nil
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
	var id string
	err := s.db.QueryRowContext(ctx, "SELECT id FROM environments WHERE code = ?", code).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("find environment: %w", err)
	}

	id, err = newID("env")
	if err != nil {
		return "", err
	}
	if _, err := s.db.ExecContext(
		ctx,
		`INSERT INTO environments (id, name, code, enabled, source, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id,
		code,
		code,
		1,
		string(model.ConfigSourceAPI),
		formatTime(now),
		formatTime(now),
	); err != nil {
		return "", fmt.Errorf("insert environment: %w", err)
	}
	return id, nil
}

func (s *agentStore) updateExistingAgent(ctx context.Context, id, regionID, runtimeType, version string, now time.Time) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE agents
         SET region_id = ?, runtime_type = ?, version = ?, last_seen_at = ?, status = ?, updated_at = ?
         WHERE id = ?`,
		regionID,
		runtimeType,
		version,
		formatTime(now),
		string(model.AgentStatusOnline),
		formatTime(now),
		id,
	)
	if err != nil {
		return fmt.Errorf("update existing agent: %w", err)
	}
	return nil
}

func (s *agentStore) replaceAgentEnvironments(ctx context.Context, agentID string, environmentIDs []string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM agent_environments WHERE agent_id = ?", agentID); err != nil {
		return fmt.Errorf("clear agent environments: %w", err)
	}
	for _, environmentID := range environmentIDs {
		if _, err := s.db.ExecContext(
			ctx,
			"INSERT INTO agent_environments (agent_id, environment_id) VALUES (?, ?)",
			agentID,
			environmentID,
		); err != nil {
			return fmt.Errorf("insert agent environment: %w", err)
		}
	}
	return nil
}

func (s *agentStore) agentEnvironmentIDs(ctx context.Context, agentID string) (*collectionlist.List[string], error) {
	rows, err := s.db.QueryContext(ctx, "SELECT environment_id FROM agent_environments WHERE agent_id = ? ORDER BY environment_id", agentID)
	if err != nil {
		return nil, fmt.Errorf("query agent environments: %w", err)
	}
	defer closeRows(rows)

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan agent environment: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agent environments: %w", err)
	}
	return collectionlist.NewList[string](ids...), nil
}
