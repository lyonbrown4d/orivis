package store

import (
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
	out.EnvironmentCodes = collectionlist.FilterMapList(
		collectionlist.NewList(params.EnvironmentCodes...),
		func(_ int, code string) (string, bool) {
			normalized := normalizeCode(code)
			return normalized, normalized != ""
		},
	).Values()

	switch {
	case out.Name == "":
		return out, wrapError(ErrInvalidInput, "agent name is required")
	case out.RegionCode == "":
		return out, wrapError(ErrInvalidInput, "region code is required")
	case out.RuntimeType == "":
		return out, wrapError(ErrInvalidInput, "runtime type is required")
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
		return "", wrapError(err, "hash agent token")
	}
	return string(hash), nil
}

func verifyAgentToken(hash, token string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(token)); err != nil {
		return wrapError(ErrUnauthorized, "invalid agent token")
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
		return time.Time{}, wrapErrorf(err, "parse time %q", value)
	}
	return t, nil
}
