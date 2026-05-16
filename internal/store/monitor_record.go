package store

import (
	"fmt"
	"time"

	"github.com/lyonbrown4d/orivis/internal/model"
)

type rowScanner interface {
	Scan(dest ...any) error
}

func scanMonitor(row rowScanner) (model.Monitor, error) {
	var rec monitorRecord
	if err := row.Scan(
		&rec.ID,
		&rec.Name,
		&rec.Type,
		&rec.Target,
		&rec.EnvironmentID,
		&rec.Enabled,
		&rec.SourceKey,
		&rec.IntervalSeconds,
		&rec.TimeoutSeconds,
		&rec.RetryCount,
		&rec.AggregationPolicy,
		&rec.Source,
		&rec.CreatedAt,
		&rec.UpdatedAt,
	); err != nil {
		return model.Monitor{}, fmt.Errorf("scan monitor: %w", err)
	}
	return rec.model()
}

type monitorRecord struct {
	ID                string
	SourceKey         string
	Name              string
	Type              string
	Target            string
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

func (r monitorRecord) model() (model.Monitor, error) {
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
