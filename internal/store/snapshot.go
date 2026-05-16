package store

import (
	"context"
	"time"

	"github.com/lyonbrown4d/orivis/internal/model"
)

type DashboardSnapshot struct {
	GeneratedAt time.Time
	Agents      []DashboardAgent
	Monitors    []DashboardMonitor
	Results     []DashboardResult
}

type DashboardAgent struct {
	ID               string
	Name             string
	RegionCode       string
	EnvironmentCodes []string
	RuntimeType      string
	Version          string
	LastSeenAt       time.Time
	Status           model.AgentStatus
}

type DashboardMonitor struct {
	ID                string
	Name              string
	Type              model.MonitorType
	Target            string
	EnvironmentCode   string
	Enabled           bool
	Interval          time.Duration
	Timeout           time.Duration
	RetryCount        int
	AggregationPolicy model.AggregationPolicy
	Source            model.ConfigSource
}

type DashboardResult struct {
	ID              string
	MonitorID       string
	AgentID         string
	AgentName       string
	RegionCode      string
	EnvironmentCode string
	Status          model.Status
	Latency         time.Duration
	ErrorMessage    string
	CheckedAt       time.Time
	CreatedAt       time.Time
}

func (s *Store) DashboardSnapshot(ctx context.Context, resultLimit int) (DashboardSnapshot, error) {
	if resultLimit <= 0 {
		resultLimit = 50
	}

	out := DashboardSnapshot{GeneratedAt: time.Now().UTC()}
	switch {
	case s == nil:
		return out, nil
	case s.memory != nil:
		return s.memoryDashboardSnapshot(resultLimit)
	case s.DB != nil:
		return s.sqlDashboardSnapshot(ctx, resultLimit)
	default:
		return out, nil
	}
}
