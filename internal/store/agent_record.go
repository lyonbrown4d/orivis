package store

import (
	"fmt"
	"strings"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/lyonbrown4d/orivis/internal/model"
	"golang.org/x/crypto/bcrypt"
)

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
