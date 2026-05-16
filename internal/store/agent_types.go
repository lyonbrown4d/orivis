package store

import "time"

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

type normalizedRegisterParams struct {
	Name             string
	Token            string
	RegionCode       string
	EnvironmentCodes []string
	RuntimeType      string
	Version          string
}

type agentCredential struct {
	ID        string
	TokenHash string
	Found     bool
}
