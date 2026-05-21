package discovery

import (
	"strings"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/hashicorp/hcl/v2/hclsimple"
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
	paths := collectionlist.FilterMapList(collectionlist.NewList(files...), func(_ int, file string) (string, bool) {
		path := strings.TrimSpace(file)
		return path, path != ""
	})
	monitors, err := collectionlist.ReduceErrList(
		paths,
		collectionlist.NewList[StaticMonitor](),
		func(out *collectionlist.List[StaticMonitor], _ int, path string) (*collectionlist.List[StaticMonitor], error) {
			loaded, err := loadStaticMonitorsHCLFile(path)
			if err != nil {
				return nil, err
			}
			out.Add(loaded...)
			return out, nil
		},
	)
	if err != nil {
		return nil, wrapError(err, "load static monitor HCL files")
	}
	return monitors.Values(), nil
}

func loadStaticMonitorsHCLFile(path string) ([]StaticMonitor, error) {
	var file staticHCLFile
	if err := hclsimple.DecodeFile(path, nil, &file); err != nil {
		return nil, wrapErrorf(err, "load static monitor HCL file %s", path)
	}

	monitors, err := collectionlist.ReduceErrList(
		collectionlist.NewList(file.Probes...),
		collectionlist.NewListWithCapacity[StaticMonitor](len(file.Probes)),
		func(out *collectionlist.List[StaticMonitor], _ int, probe staticHCLProbe) (*collectionlist.List[StaticMonitor], error) {
			monitor, err := probe.staticMonitor()
			if err != nil {
				return nil, wrapErrorf(err, "decode static monitor HCL probe %q", probe.Name)
			}
			out.Add(monitor)
			return out, nil
		},
	)
	if err != nil {
		return nil, wrapErrorf(err, "decode static monitor HCL file %s", path)
	}
	return monitors.Values(), nil
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
		return 0, wrapErrorf(err, "parse HCL duration %q", value)
	}
	return duration, nil
}
