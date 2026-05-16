package api

import (
	"crypto/subtle"
	"errors"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/protocol"
	"github.com/lyonbrown4d/orivis/internal/store"
)

type agentRegisterInput struct {
	Body protocol.AgentRegisterRequest `json:"body"`
}

type agentRegisterOutput struct {
	Body protocol.AgentRegisterResponse `json:"body"`
}

type agentHeartbeatInput struct {
	Body protocol.AgentHeartbeatRequest `json:"body"`
}

type agentHeartbeatOutput struct {
	Body protocol.AgentHeartbeatResponse `json:"body"`
}

type agentTasksInput struct {
	AgentID string `query:"agent_id"        validate:"required"`
	Token   string `query:"token,omitempty"`
}

type agentTasksOutput struct {
	Body protocol.AgentTasksResponse `json:"body"`
}

type agentMonitorSyncInput struct {
	Body protocol.AgentMonitorSyncRequest `json:"body"`
}

type agentMonitorSyncOutput struct {
	Body protocol.AgentMonitorSyncResponse `json:"body"`
}

type agentResultsInput struct {
	Body protocol.AgentResultRequest `json:"body"`
}

func (e *agentEndpoint) verifyBootstrapToken(token string) error {
	expected := strings.TrimSpace(e.cfg.Auth.Agent.Token)
	if expected == "" {
		return nil
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
		return store.ErrUnauthorized
	}
	return nil
}

func apiError(err error) error {
	switch {
	case errors.Is(err, store.ErrInvalidInput):
		return huma.Error400BadRequest(err.Error(), err)
	case errors.Is(err, store.ErrUnauthorized):
		return huma.Error401Unauthorized("unauthorized", err)
	case errors.Is(err, store.ErrNotFound):
		return huma.Error404NotFound(err.Error(), err)
	default:
		return huma.Error500InternalServerError("internal server error", err)
	}
}

func modelStatus(value string) model.Status {
	return model.Status(normalizeProtocolString(value))
}

func normalizeProtocolString(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func protocolEnabled(value *bool) bool {
	if value == nil {
		return true
	}
	return *value
}
