package protocol

import "time"

type AgentRegisterRequest struct {
	Name             string   `json:"name" validate:"required"`
	Token            string   `json:"token,omitempty"`
	RegionCode       string   `json:"region_code" validate:"required"`
	EnvironmentCodes []string `json:"environment_codes,omitempty"`
	RuntimeType      string   `json:"runtime_type" validate:"required"`
	Version          string   `json:"version,omitempty"`
}

type AgentRegisterResponse struct {
	AgentID    string    `json:"agent_id"`
	RegionID   string    `json:"region_id"`
	Status     string    `json:"status"`
	ServerTime time.Time `json:"server_time"`
}

type AgentHeartbeatRequest struct {
	AgentID string    `json:"agent_id" validate:"required"`
	Token   string    `json:"token,omitempty"`
	Version string    `json:"version,omitempty"`
	SentAt  time.Time `json:"sent_at,omitempty"`
}

type AgentHeartbeatResponse struct {
	AgentID    string    `json:"agent_id"`
	Status     string    `json:"status"`
	ServerTime time.Time `json:"server_time"`
}

type AgentTasksRequest struct {
	AgentID string `json:"agent_id"`
	Token   string `json:"token,omitempty"`
}

type AgentTask struct {
	ID             string `json:"id"`
	MonitorID      string `json:"monitor_id"`
	Type           string `json:"type"`
	Target         string `json:"target"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

type AgentTasksResponse struct {
	Tasks []AgentTask `json:"tasks"`
}

type AgentResultRequest struct {
	AgentID      string    `json:"agent_id" validate:"required"`
	Token        string    `json:"token,omitempty"`
	MonitorID    string    `json:"monitor_id" validate:"required"`
	Status       string    `json:"status" validate:"required"`
	LatencyMS    int64     `json:"latency_ms,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
	CheckedAt    time.Time `json:"checked_at,omitempty"`
	RawDetail    []byte    `json:"raw_detail,omitempty"`
}
