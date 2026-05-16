package model

import (
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
)

type Status string

const (
	StatusUp       Status = "up"
	StatusDown     Status = "down"
	StatusDegraded Status = "degraded"
	StatusUnknown  Status = "unknown"
)

type AgentStatus string

const (
	AgentStatusOnline   AgentStatus = "online"
	AgentStatusOffline  AgentStatus = "offline"
	AgentStatusDisabled AgentStatus = "disabled"
)

type MonitorType string

const (
	MonitorHTTP     MonitorType = "http"
	MonitorTCP      MonitorType = "tcp"
	MonitorPing     MonitorType = "ping"
	MonitorDNS      MonitorType = "dns"
	MonitorTLS      MonitorType = "tls"
	MonitorRedis    MonitorType = "redis"
	MonitorDatabase MonitorType = "database"
	MonitorSQLite   MonitorType = "sqlite"
	MonitorMySQL    MonitorType = "mysql"
	MonitorPostgres MonitorType = "postgres"
)

type AggregationPolicy string

const (
	AggregationAllDown      AggregationPolicy = "all_down"
	AggregationAnyDown      AggregationPolicy = "any_down"
	AggregationMajorityDown AggregationPolicy = "majority_down"
)

type ConfigSource string

const (
	ConfigSourceUI   ConfigSource = "ui"
	ConfigSourceFile ConfigSource = "file"
	ConfigSourceAPI  ConfigSource = "api"
)

type Environment struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Code        string       `json:"code"`
	Description string       `json:"description,omitempty"`
	Enabled     bool         `json:"enabled"`
	Source      ConfigSource `json:"source"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

type Region struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Code        string       `json:"code"`
	Description string       `json:"description,omitempty"`
	Enabled     bool         `json:"enabled"`
	Source      ConfigSource `json:"source"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

type Agent struct {
	ID             string                       `json:"id"`
	Name           string                       `json:"name"`
	TokenHash      string                       `json:"-"`
	RegionID       string                       `json:"region_id"`
	EnvironmentIDs *collectionlist.List[string] `json:"-"`
	RuntimeType    string                       `json:"runtime_type"`
	Version        string                       `json:"version"`
	LastSeenAt     time.Time                    `json:"last_seen_at"`
	Status         AgentStatus                  `json:"status"`
	Source         ConfigSource                 `json:"source"`
	CreatedAt      time.Time                    `json:"created_at"`
	UpdatedAt      time.Time                    `json:"updated_at"`
}

type Monitor struct {
	ID                string            `json:"id"`
	SourceKey         string            `json:"source_key,omitempty"`
	Name              string            `json:"name"`
	Type              MonitorType       `json:"type"`
	Target            string            `json:"target"`
	EnvironmentID     string            `json:"environment_id"`
	Enabled           bool              `json:"enabled"`
	Interval          time.Duration     `json:"interval"`
	Timeout           time.Duration     `json:"timeout"`
	RetryCount        int               `json:"retry_count"`
	AggregationPolicy AggregationPolicy `json:"aggregation_policy"`
	Source            ConfigSource      `json:"source"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

type ProbeResult struct {
	ID            string        `json:"id"`
	MonitorID     string        `json:"monitor_id"`
	AgentID       string        `json:"agent_id"`
	RegionID      string        `json:"region_id"`
	EnvironmentID string        `json:"environment_id"`
	Status        Status        `json:"status"`
	Latency       time.Duration `json:"latency"`
	ErrorMessage  string        `json:"error_message,omitempty"`
	CheckedAt     time.Time     `json:"checked_at"`
	RawDetail     []byte        `json:"raw_detail,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
}
