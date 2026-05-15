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

type MonitorType string

const (
	MonitorHTTP MonitorType = "http"
	MonitorTCP  MonitorType = "tcp"
	MonitorPing MonitorType = "ping"
	MonitorDNS  MonitorType = "dns"
	MonitorTLS  MonitorType = "tls"
)

type Environment struct {
	ID          string
	Name        string
	Code        string
	Description string
	Enabled     bool
}

type Region struct {
	ID          string
	Name        string
	Code        string
	Description string
	Enabled     bool
}

type Agent struct {
	ID             string
	Name           string
	RegionID       string
	EnvironmentIDs *collectionlist.List[string]
	RuntimeType    string
	Version        string
	LastSeenAt     time.Time
	Status         Status
}

type Monitor struct {
	ID                string
	Name              string
	Type              MonitorType
	Target            string
	EnvironmentID     string
	Enabled           bool
	Interval          time.Duration
	Timeout           time.Duration
	RetryCount        int
	AggregationPolicy string
}

type ProbeResult struct {
	ID            string
	MonitorID     string
	AgentID       string
	RegionID      string
	EnvironmentID string
	Status        Status
	Latency       time.Duration
	ErrorMessage  string
	CheckedAt     time.Time
	RawDetail     []byte
}
