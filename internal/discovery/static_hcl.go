package discovery

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/samber/oops"
)

type staticHCLFile struct {
	Probes []staticHCLProbe `hcl:"probe,block"`
}

type staticHCLProbe struct {
	Type              string `hcl:"type,label"`
	Name              string `hcl:"name,label"`
	SourceKey         string `hcl:"source_key,optional"`
	Target            string `hcl:"target"`
	GroupName         string `hcl:"group,optional"`
	EnvironmentCode   string `hcl:"environment,optional"`
	Enabled           *bool  `hcl:"enabled,optional"`
	Interval          string `hcl:"interval,optional"`
	Timeout           string `hcl:"timeout,optional"`
	RetryCount        int    `hcl:"retry_count,optional"`
	AggregationPolicy string `hcl:"aggregation,optional"`
}

func LoadStaticMonitorsHCL(files []string) ([]StaticMonitor, error) {
	monitors := make([]StaticMonitor, 0)
	for _, file := range files {
		path := strings.TrimSpace(file)
		if path == "" {
			continue
		}
		loaded, err := loadStaticMonitorsHCLFile(path)
		if err != nil {
			return nil, err
		}
		monitors = append(monitors, loaded...)
	}
	return monitors, nil
}

func loadStaticMonitorsHCLFile(path string) ([]StaticMonitor, error) {
	var file staticHCLFile
	if err := hclsimple.DecodeFile(path, nil, &file); err != nil {
		return nil, fmt.Errorf("%w", oops.Wrapf(err, "load static monitor HCL file %s", path))
	}

	monitors := make([]StaticMonitor, 0, len(file.Probes))
	for index := range file.Probes {
		monitor, err := file.Probes[index].staticMonitor()
		if err != nil {
			return nil, fmt.Errorf("%w", oops.Wrapf(err, "decode static monitor HCL probe %q", file.Probes[index].Name))
		}
		monitors = append(monitors, monitor)
	}
	return monitors, nil
}

func (probe staticHCLProbe) staticMonitor() (StaticMonitor, error) {
	interval, err := parseHCLDuration(probe.Interval)
	if err != nil {
		return StaticMonitor{}, err
	}
	timeout, err := parseHCLDuration(probe.Timeout)
	if err != nil {
		return StaticMonitor{}, err
	}

	sourceKey := strings.TrimSpace(probe.SourceKey)
	if sourceKey == "" {
		sourceKey = "static:hcl:" + strings.TrimSpace(probe.Type) + ":" + strings.TrimSpace(probe.Name)
	}

	return StaticMonitor{
		SourceKey:         sourceKey,
		Name:              strings.TrimSpace(probe.Name),
		Type:              strings.TrimSpace(probe.Type),
		Target:            strings.TrimSpace(probe.Target),
		GroupName:         strings.TrimSpace(probe.GroupName),
		EnvironmentCode:   strings.TrimSpace(probe.EnvironmentCode),
		Enabled:           probe.Enabled,
		Interval:          interval,
		Timeout:           timeout,
		RetryCount:        probe.RetryCount,
		AggregationPolicy: strings.TrimSpace(probe.AggregationPolicy),
	}, nil
}

func parseHCLDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%w", oops.Wrapf(err, "parse HCL duration %q", value))
	}
	return duration, nil
}
