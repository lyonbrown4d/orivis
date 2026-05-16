package store

import (
	"fmt"
	"strings"
	"time"

	"github.com/lyonbrown4d/orivis/internal/model"
)

type CreateMonitorParams struct {
	SourceKey         string
	Name              string
	Type              model.MonitorType
	Target            string
	GroupName         string
	EnvironmentID     string
	Enabled           bool
	Interval          time.Duration
	Timeout           time.Duration
	RetryCount        int
	AggregationPolicy model.AggregationPolicy
	Source            model.ConfigSource
}

type UpsertDiscoveredMonitorParams struct {
	SourceKey         string
	Name              string
	Type              model.MonitorType
	Target            string
	GroupName         string
	EnvironmentID     string
	Enabled           bool
	Interval          time.Duration
	Timeout           time.Duration
	RetryCount        int
	AggregationPolicy model.AggregationPolicy
}

type createMonitorParams struct {
	SourceKey         string
	Name              string
	Type              model.MonitorType
	Target            string
	GroupName         string
	EnvironmentID     string
	Enabled           bool
	Interval          time.Duration
	Timeout           time.Duration
	RetryCount        int
	AggregationPolicy model.AggregationPolicy
	Source            model.ConfigSource
}

func normalizeCreateMonitorParams(params CreateMonitorParams) (createMonitorParams, error) {
	out := createMonitorParams{
		SourceKey:         strings.TrimSpace(params.SourceKey),
		Name:              strings.TrimSpace(params.Name),
		Type:              params.Type,
		Target:            strings.TrimSpace(params.Target),
		GroupName:         strings.TrimSpace(params.GroupName),
		EnvironmentID:     strings.TrimSpace(params.EnvironmentID),
		Enabled:           params.Enabled,
		Interval:          params.Interval,
		Timeout:           params.Timeout,
		RetryCount:        max(0, params.RetryCount),
		AggregationPolicy: params.AggregationPolicy,
		Source:            params.Source,
	}
	applyMonitorDefaults(&out)
	return validateMonitorParams(out)
}

func normalizeDiscoveredMonitorParams(params UpsertDiscoveredMonitorParams) (createMonitorParams, error) {
	out, err := normalizeCreateMonitorParams(CreateMonitorParams{
		SourceKey:         params.SourceKey,
		Name:              params.Name,
		Type:              params.Type,
		Target:            params.Target,
		GroupName:         params.GroupName,
		EnvironmentID:     params.EnvironmentID,
		Enabled:           params.Enabled,
		Interval:          params.Interval,
		Timeout:           params.Timeout,
		RetryCount:        params.RetryCount,
		AggregationPolicy: params.AggregationPolicy,
		Source:            model.ConfigSourceAPI,
	})
	if err != nil {
		return out, err
	}
	if out.SourceKey == "" {
		return out, fmt.Errorf("%w: monitor source key is required", ErrInvalidInput)
	}
	return out, nil
}

func createMonitorParamsToPublic(params createMonitorParams) CreateMonitorParams {
	return CreateMonitorParams(params)
}

func applyMonitorDefaults(params *createMonitorParams) {
	if params.Interval <= 0 {
		params.Interval = 30 * time.Second
	}
	if params.Timeout <= 0 {
		params.Timeout = 5 * time.Second
	}
	if params.AggregationPolicy == "" {
		params.AggregationPolicy = model.AggregationMajorityDown
	}
	if params.Source == "" {
		params.Source = model.ConfigSourceAPI
	}
}

func validateMonitorParams(params createMonitorParams) (createMonitorParams, error) {
	switch {
	case params.Name == "":
		return params, fmt.Errorf("%w: monitor name is required", ErrInvalidInput)
	case params.Type == "":
		return params, fmt.Errorf("%w: monitor type is required", ErrInvalidInput)
	case params.Target == "":
		return params, fmt.Errorf("%w: monitor target is required", ErrInvalidInput)
	case params.EnvironmentID == "":
		return params, fmt.Errorf("%w: environment id is required", ErrInvalidInput)
	default:
		return params, nil
	}
}
