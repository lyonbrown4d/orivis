package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/lyonbrown4d/orivis/internal/discovery"
	"github.com/samber/mo"
	"github.com/samber/oops"
)

type agentHCLFile struct {
	Server    *agentHCLServer    `hcl:"server,block"`
	Agent     *agentHCLAgent     `hcl:"agent,block"`
	Runtime   string             `hcl:"runtime,optional"`
	Poll      *agentHCLPoll      `hcl:"poll,block"`
	Log       *agentHCLLog       `hcl:"log,block"`
	Discovery *agentHCLDiscovery `hcl:"discovery,block"`
}

type agentHCLServer struct {
	URL string `hcl:"url,optional"`
}

type agentHCLAgent struct {
	Name         string   `hcl:"name,optional"`
	Token        string   `hcl:"token,optional"`
	Region       string   `hcl:"region,optional"`
	Environments []string `hcl:"environments,optional"`
}

type agentHCLPoll struct {
	Interval string `hcl:"interval,optional"`
}

type agentHCLLog struct {
	Level string `hcl:"level,optional"`
}

type agentHCLDiscovery struct {
	Static *agentHCLStatic `hcl:"static,block"`
	Docker *agentHCLDocker `hcl:"docker,block"`
	Probes []agentHCLProbe `hcl:"probe,block"`
}

type agentHCLStatic struct {
	Enabled  *bool           `hcl:"enabled,optional"`
	HCLFiles []string        `hcl:"hcl_files,optional"`
	Probes   []agentHCLProbe `hcl:"probe,block"`
}

type agentHCLDocker struct {
	Enabled *bool  `hcl:"enabled,optional"`
	Mode    string `hcl:"mode,optional"`
}

type agentHCLProbe struct {
	Type              string `hcl:"type,label"`
	Name              string `hcl:"name,label"`
	SourceKey         string `hcl:"source_key,optional"`
	Target            string `hcl:"target"`
	GroupName         string `hcl:"group,optional"`
	EnvironmentCode   string `hcl:"environment,optional"`
	Enabled           *bool  `hcl:"enabled,optional"`
	Interval          string `hcl:"interval,optional"`
	Timeout           string `hcl:"timeout,optional"`
	RetryCount        *int   `hcl:"retry_count,optional"`
	AggregationPolicy string `hcl:"aggregation,optional"`
}

type agentHCLParser struct{}

func agentHCLFileParser() agentHCLParser {
	return agentHCLParser{}
}

func (agentHCLParser) Unmarshal(raw []byte) (map[string]any, error) {
	values, err := decodeAgentHCL("agent.hcl", raw)
	if err != nil {
		return nil, err
	}
	return values, nil
}

func (agentHCLParser) Marshal(map[string]any) ([]byte, error) {
	return nil, errors.New("agent HCL marshal is not supported")
}

func decodeAgentHCL(filename string, raw []byte) (map[string]any, error) {
	var file agentHCLFile
	if err := hclsimple.Decode(filename, raw, nil, &file); err != nil {
		return nil, fmt.Errorf("%w", oops.Wrapf(err, "load agent HCL config %s", filename))
	}
	return file.defaults()
}

func (file agentHCLFile) defaults() (map[string]any, error) {
	values := make(map[string]any)
	file.applyServer(values)
	file.applyAgent(values)
	file.applyRuntime(values)
	file.applyPoll(values)
	file.applyLog(values)
	if err := file.applyDiscovery(values); err != nil {
		return nil, err
	}
	return values, nil
}

func (file agentHCLFile) applyServer(values map[string]any) {
	if file.Server == nil {
		return
	}
	setString(values, "server.url", file.Server.URL)
}

func (file agentHCLFile) applyAgent(values map[string]any) {
	if file.Agent == nil {
		return
	}
	setString(values, "agent.name", file.Agent.Name)
	setString(values, "agent.token", file.Agent.Token)
	setString(values, "agent.region", file.Agent.Region)
	if len(file.Agent.Environments) > 0 {
		setValue(values, "agent.environments", file.Agent.Environments)
	}
}

func (file agentHCLFile) applyRuntime(values map[string]any) {
	setString(values, "runtime", file.Runtime)
}

func (file agentHCLFile) applyPoll(values map[string]any) {
	if file.Poll == nil {
		return
	}
	setString(values, "poll.interval", file.Poll.Interval)
}

func (file agentHCLFile) applyLog(values map[string]any) {
	if file.Log == nil {
		return
	}
	setString(values, "log.level", file.Log.Level)
}

func (file agentHCLFile) applyDiscovery(values map[string]any) error {
	if file.Discovery == nil {
		return nil
	}
	file.Discovery.applyStatic(values)
	file.Discovery.applyDocker(values)
	monitors, err := file.Discovery.staticMonitors()
	if err != nil {
		return err
	}
	if len(monitors) > 0 {
		setValue(values, "discovery.static.monitors", monitors)
	}
	return nil
}

func (discoveryConfig agentHCLDiscovery) applyStatic(values map[string]any) {
	if discoveryConfig.Static == nil {
		return
	}
	setOptional(values, "discovery.static.enabled", discoveryConfig.Static.Enabled)
	if len(discoveryConfig.Static.HCLFiles) > 0 {
		setValue(values, "discovery.static.hcl_files", discoveryConfig.Static.HCLFiles)
	}
}

func (discoveryConfig agentHCLDiscovery) applyDocker(values map[string]any) {
	if discoveryConfig.Docker == nil {
		return
	}
	setOptional(values, "discovery.docker.enabled", discoveryConfig.Docker.Enabled)
	setString(values, "discovery.docker.mode", discoveryConfig.Docker.Mode)
}

func (discoveryConfig agentHCLDiscovery) staticMonitors() ([]discovery.StaticMonitor, error) {
	probes := discoveryConfig.probes()
	monitors, err := collectionlist.ReduceErrList(
		probes,
		collectionlist.NewListWithCapacity[discovery.StaticMonitor](probes.Len()),
		func(acc *collectionlist.List[discovery.StaticMonitor], _ int, probe agentHCLProbe) (*collectionlist.List[discovery.StaticMonitor], error) {
			monitor, err := probe.staticMonitor()
			if err != nil {
				return nil, fmt.Errorf("%w", oops.Wrapf(err, "decode agent HCL probe %q", probe.Name))
			}
			acc.Add(monitor)
			return acc, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("build hcl static monitors: %w", err)
	}
	return monitors.Values(), nil
}

func (discoveryConfig agentHCLDiscovery) probes() *collectionlist.List[agentHCLProbe] {
	probes := collectionlist.NewList(discoveryConfig.Probes...)
	if discoveryConfig.Static != nil {
		probes.Add(discoveryConfig.Static.Probes...)
	}
	return probes
}

func (probe agentHCLProbe) staticMonitor() (discovery.StaticMonitor, error) {
	interval, err := parseAgentHCLDuration(probe.Interval)
	if err != nil {
		return discovery.StaticMonitor{}, err
	}
	timeout, err := parseAgentHCLDuration(probe.Timeout)
	if err != nil {
		return discovery.StaticMonitor{}, err
	}

	sourceKey := strings.TrimSpace(probe.SourceKey)
	if sourceKey == "" {
		sourceKey = "static:hcl:" + strings.TrimSpace(probe.Type) + ":" + strings.TrimSpace(probe.Name)
	}

	return discovery.StaticMonitor{
		SourceKey:         sourceKey,
		Name:              strings.TrimSpace(probe.Name),
		Type:              strings.TrimSpace(probe.Type),
		Target:            strings.TrimSpace(probe.Target),
		GroupName:         strings.TrimSpace(probe.GroupName),
		EnvironmentCode:   strings.TrimSpace(probe.EnvironmentCode),
		Enabled:           probe.Enabled,
		Interval:          interval,
		Timeout:           timeout,
		RetryCount:        mo.PointerToOption(probe.RetryCount).OrElse(0),
		AggregationPolicy: strings.TrimSpace(probe.AggregationPolicy),
	}, nil
}

func setOptional[T any](values map[string]any, key string, value *T) {
	mo.PointerToOption(value).ForEach(func(value T) {
		setValue(values, key, value)
	})
}

func parseAgentHCLDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%w", oops.Wrapf(err, "parse agent HCL duration %q", value))
	}
	return duration, nil
}

func setString(values map[string]any, key, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		setValue(values, key, value)
	}
}

func setValue(values map[string]any, key string, value any) {
	parts := strings.Split(key, ".")
	cursor := values
	for _, part := range parts[:len(parts)-1] {
		next, ok := cursor[part].(map[string]any)
		if !ok {
			next = make(map[string]any)
			cursor[part] = next
		}
		cursor = next
	}
	cursor[parts[len(parts)-1]] = value
}
