package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/dbx"
	"github.com/lyonbrown4d/orivis/internal/model"
	"golang.org/x/crypto/bcrypt"
)

type AgentStore interface {
	Register(ctx context.Context, params RegisterAgentParams) (model.Agent, error)
	RecordHeartbeat(ctx context.Context, params AgentHeartbeatParams) (model.Agent, error)
	Authenticate(ctx context.Context, agentID, token string) (model.Agent, error)
	Get(ctx context.Context, id string) (model.Agent, error)
}

type RegisterAgentParams struct {
	Name             string
	Token            string
	RegionCode       string
	EnvironmentCodes []string
	RuntimeType      string
	Version          string
}

type AgentHeartbeatParams struct {
	AgentID string
	Token   string
	Version string
	SeenAt  time.Time
}

type agentStore struct {
	db *dbx.DB
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

	var existingID, existingHash string
	err = s.db.QueryRowContext(ctx, "SELECT id, token_hash FROM agents WHERE name = ?", normalized.Name).Scan(&existingID, &existingHash)
	switch {
	case err == nil:
		if err := verifyAgentToken(existingHash, normalized.Token); err != nil {
			return model.Agent{}, err
		}
		if err := s.updateExistingAgent(ctx, existingID, regionID, normalized.RuntimeType, normalized.Version, now); err != nil {
			return model.Agent{}, err
		}
		if err := s.replaceAgentEnvironments(ctx, existingID, environmentIDs); err != nil {
			return model.Agent{}, err
		}
		return s.Get(ctx, existingID)
	case errors.Is(err, sql.ErrNoRows):
		id, err := newID("agt")
		if err != nil {
			return model.Agent{}, err
		}
		tokenHash, err := hashAgentToken(normalized.Token)
		if err != nil {
			return model.Agent{}, err
		}
		if _, err := s.db.ExecContext(
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
		); err != nil {
			return model.Agent{}, fmt.Errorf("insert agent: %w", err)
		}
		if err := s.replaceAgentEnvironments(ctx, id, environmentIDs); err != nil {
			return model.Agent{}, err
		}
		return s.Get(ctx, id)
	default:
		return model.Agent{}, fmt.Errorf("find agent by name: %w", err)
	}
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

	if _, err := s.db.ExecContext(
		ctx,
		`UPDATE agents
         SET version = ?, last_seen_at = ?, status = ?, updated_at = ?
         WHERE id = ?`,
		strings.TrimSpace(params.Version),
		formatTime(seenAt),
		string(model.AgentStatusOnline),
		formatTime(time.Now().UTC()),
		agentID,
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
			return model.Agent{}, fmt.Errorf("%w: agent %s", ErrNotFound, id)
		}
		return model.Agent{}, fmt.Errorf("get agent: %w", err)
	}

	environmentIDs, err := s.agentEnvironmentIDs(ctx, id)
	if err != nil {
		return model.Agent{}, err
	}
	return rec.model(environmentIDs)
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
	defer rows.Close()

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

type normalizedRegisterParams struct {
	Name             string
	Token            string
	RegionCode       string
	EnvironmentCodes []string
	RuntimeType      string
	Version          string
}

func normalizeRegisterParams(params RegisterAgentParams) (normalizedRegisterParams, error) {
	out := normalizedRegisterParams{
		Name:        strings.TrimSpace(params.Name),
		Token:       params.Token,
		RegionCode:  normalizeCode(params.RegionCode),
		RuntimeType: strings.TrimSpace(params.RuntimeType),
		Version:     strings.TrimSpace(params.Version),
	}
	for _, code := range params.EnvironmentCodes {
		if normalized := normalizeCode(code); normalized != "" {
			out.EnvironmentCodes = append(out.EnvironmentCodes, normalized)
		}
	}

	switch {
	case out.Name == "":
		return out, fmt.Errorf("%w: agent name is required", ErrInvalidInput)
	case out.RegionCode == "":
		return out, fmt.Errorf("%w: region code is required", ErrInvalidInput)
	case out.RuntimeType == "":
		return out, fmt.Errorf("%w: runtime type is required", ErrInvalidInput)
	default:
		return out, nil
	}
}

type agentRecord struct {
	ID          string
	Name        string
	TokenHash   string
	RegionID    string
	RuntimeType string
	Version     string
	LastSeenAt  string
	Status      string
	Source      string
	CreatedAt   string
	UpdatedAt   string
}

func (r agentRecord) model(environmentIDs *collectionlist.List[string]) (model.Agent, error) {
	lastSeenAt, err := parseTime(r.LastSeenAt)
	if err != nil {
		return model.Agent{}, err
	}
	createdAt, err := parseTime(r.CreatedAt)
	if err != nil {
		return model.Agent{}, err
	}
	updatedAt, err := parseTime(r.UpdatedAt)
	if err != nil {
		return model.Agent{}, err
	}

	return model.Agent{
		ID:             r.ID,
		Name:           r.Name,
		TokenHash:      r.TokenHash,
		RegionID:       r.RegionID,
		EnvironmentIDs: environmentIDs,
		RuntimeType:    r.RuntimeType,
		Version:        r.Version,
		LastSeenAt:     lastSeenAt,
		Status:         model.AgentStatus(r.Status),
		Source:         model.ConfigSource(r.Source),
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}, nil
}

func hashAgentToken(token string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash agent token: %w", err)
	}
	return string(hash), nil
}

func verifyAgentToken(hash, token string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(token)); err != nil {
		return fmt.Errorf("%w: invalid agent token", ErrUnauthorized)
	}
	return nil
}

func normalizeCode(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", value, err)
	}
	return t, nil
}
