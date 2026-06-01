package store

import (
	"context"
	"time"

	"github.com/lyonbrown4d/orivis/internal/model"
)

type DashboardSnapshot struct {
	GeneratedAt   time.Time
	Agents        []DashboardAgent
	Monitors      []DashboardMonitor
	Results       []DashboardResult
	Notifications []DashboardNotification
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
	SourceKey         string
	Name              string
	Type              model.MonitorType
	Target            string
	GroupName         string
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
	GroupName       string
	Status          model.Status
	Latency         time.Duration
	ErrorMessage    string
	CheckedAt       time.Time
	CreatedAt       time.Time
}

type DashboardMonitorDetail struct {
	Monitor       DashboardMonitor
	Results       []DashboardResult
	Notifications []DashboardNotification
}

func (s *Store) DashboardSnapshot(ctx context.Context, resultLimit int) (DashboardSnapshot, error) {
	if resultLimit <= 0 {
		resultLimit = 50
	}

	out := DashboardSnapshot{GeneratedAt: time.Now().UTC()}
	switch {
	case s == nil:
		return out, nil
	case s.DB != nil:
		return s.sqlDashboardSnapshot(ctx, resultLimit)
	default:
		return out, nil
	}
}

func (s *Store) DashboardMonitorDetail(ctx context.Context, monitorID string, resultLimit, notificationLimit int) (DashboardMonitorDetail, error) {
	if resultLimit <= 0 {
		resultLimit = 50
	}
	if notificationLimit <= 0 {
		notificationLimit = 20
	}

	out := DashboardMonitorDetail{}
	switch {
	case s == nil:
		return out, nil
	case s.DB != nil:
		return s.sqlDashboardMonitorDetail(ctx, monitorID, resultLimit, notificationLimit)
	default:
		return out, nil
	}
}
