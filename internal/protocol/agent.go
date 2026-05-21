package protocol

import "time"

type AgentRegisterRequest struct {
	Name             string   `json:"name"                        validate:"required"`
	Token            string   `json:"token,omitempty"`
	RegionCode       string   `json:"region_code"                 validate:"required"`
	EnvironmentCodes []string `json:"environment_codes,omitempty"`
	RuntimeType      string   `json:"runtime_type"                validate:"required"`
	Version          string   `json:"version,omitempty"`
}

type AgentRegisterResponse struct {
	AgentID    string    `json:"agent_id"`
	RegionID   string    `json:"region_id"`
	Status     string    `json:"status"`
	ServerTime time.Time `json:"server_time"`
}

type AgentHeartbeatRequest struct {
	AgentID string    `json:"agent_id"          validate:"required"`
	Token   string    `json:"token,omitempty"`
	Version string    `json:"version,omitempty"`
	SentAt  time.Time `json:"sent_at,omitzero"`
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
	ID              string `json:"id"`
	MonitorID       string `json:"monitor_id"`
	Type            string `json:"type"`
	Target          string `json:"target"`
	IntervalSeconds int    `json:"interval_seconds,omitempty"`
	TimeoutSeconds  int    `json:"timeout_seconds,omitempty"`
}

type AgentTasksResponse struct {
	Tasks []AgentTask `json:"tasks"`
}

type AgentMonitorSyncRequest struct {
	AgentID  string                   `json:"agent_id"        validate:"required"`
	Token    string                   `json:"token,omitempty"`
	Monitors []AgentDiscoveredMonitor `json:"monitors"`
}

type AgentDiscoveredMonitor struct {
	SourceKey         string `json:"source_key"                   validate:"required"`
	Name              string `json:"name"                         validate:"required"`
	Type              string `json:"type"                         validate:"required"`
	Target            string `json:"target"                       validate:"required"`
	GroupName         string `json:"group,omitempty"`
	EnvironmentCode   string `json:"environment_code,omitempty"`
	Enabled           *bool  `json:"enabled,omitempty"`
	IntervalSeconds   int    `json:"interval_seconds,omitempty"`
	TimeoutSeconds    int    `json:"timeout_seconds,omitempty"`
	RetryCount        int    `json:"retry_count,omitempty"`
	AggregationPolicy string `json:"aggregation_policy,omitempty"`
}

type AgentMonitorSyncResponse struct {
	Synced int `json:"synced"`
}

type AgentResultRequest struct {
	AgentID      string    `json:"agent_id"                validate:"required"`
	Token        string    `json:"token,omitempty"`
	ResultID     string    `json:"result_id,omitempty"`
	MonitorID    string    `json:"monitor_id"              validate:"required"`
	Status       string    `json:"status"                  validate:"required"`
	LatencyMS    int64     `json:"latency_ms,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
	CheckedAt    time.Time `json:"checked_at,omitzero"`
	RawDetail    []byte    `json:"raw_detail,omitempty"`
}

type AgentResult struct {
	ResultID     string    `json:"result_id,omitempty"`
	MonitorID    string    `json:"monitor_id"              validate:"required"`
	Status       string    `json:"status"                  validate:"required"`
	LatencyMS    int64     `json:"latency_ms,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
	CheckedAt    time.Time `json:"checked_at,omitzero"`
	RawDetail    []byte    `json:"raw_detail,omitempty"`
}

type AgentResultBatchRequest struct {
	AgentID string        `json:"agent_id"        validate:"required"`
	Token   string        `json:"token,omitempty"`
	Results []AgentResult `json:"results"         validate:"required,min=1"`
}

type AgentResultBatchResponse struct {
	Accepted int `json:"accepted"`
}

func AgentResultFromRequest(req AgentResultRequest) AgentResult {
	return AgentResult{
		ResultID:     req.ResultID,
		MonitorID:    req.MonitorID,
		Status:       req.Status,
		LatencyMS:    req.LatencyMS,
		ErrorMessage: req.ErrorMessage,
		CheckedAt:    req.CheckedAt,
		RawDetail:    req.RawDetail,
	}
}
