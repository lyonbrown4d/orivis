package store

import (
	"time"

	"github.com/lyonbrown4d/orivis/internal/model"
)

type monitorRecord struct {
	ID                string
	SourceKey         string
	Name              string
	Type              string
	Target            string
	GroupName         string
	EnvironmentID     string
	Enabled           int
	IntervalSeconds   int
	TimeoutSeconds    int
	RetryCount        int
	AggregationPolicy string
	Source            string
	CreatedAt         string
	UpdatedAt         string
}

func (r *monitorRecord) model() (model.Monitor, error) {
	createdAt, err := parseTime(r.CreatedAt)
	if err != nil {
		return model.Monitor{}, err
	}
	updatedAt, err := parseTime(r.UpdatedAt)
	if err != nil {
		return model.Monitor{}, err
	}
	return model.Monitor{
		ID:                r.ID,
		SourceKey:         r.SourceKey,
		Name:              r.Name,
		Type:              model.MonitorType(r.Type),
		Target:            r.Target,
		GroupName:         r.GroupName,
		EnvironmentID:     r.EnvironmentID,
		Enabled:           r.Enabled == 1,
		Interval:          time.Duration(r.IntervalSeconds) * time.Second,
		Timeout:           time.Duration(r.TimeoutSeconds) * time.Second,
		RetryCount:        r.RetryCount,
		AggregationPolicy: model.AggregationPolicy(r.AggregationPolicy),
		Source:            model.ConfigSource(r.Source),
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
	}, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
