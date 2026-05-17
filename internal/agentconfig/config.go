package config

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/configx"
	"github.com/lyonbrown4d/orivis/internal/discovery"
	"github.com/samber/lo"
	"github.com/spf13/pflag"
)

type Config struct {
	Server struct {
		URL string `mapstructure:"url" validate:"required,url"`
	} `mapstructure:"server"`
	Agent struct {
		Name         string   `mapstructure:"name"         validate:"required"`
		Token        string   `mapstructure:"token"`
		Region       string   `mapstructure:"region"       validate:"required"`
		Environments []string `mapstructure:"environments"`
	} `mapstructure:"agent"`
	Runtime string `mapstructure:"runtime" validate:"required"`
	Poll    struct {
		Interval time.Duration `mapstructure:"interval" validate:"required"`
	} `mapstructure:"poll"`
	Discovery struct {
		Static struct {
			Monitor  discovery.StaticMonitor   `mapstructure:"monitor"`
			Enabled  bool                      `mapstructure:"enabled"`
			HCLFiles []string                  `mapstructure:"hcl_files"`
			Monitors []discovery.StaticMonitor `mapstructure:"monitors"`
		} `mapstructure:"static"`
		Docker struct {
			Enabled bool   `mapstructure:"enabled"`
			Mode    string `mapstructure:"mode"`
		} `mapstructure:"docker"`
	} `mapstructure:"discovery"`
	Log struct {
		Level string `mapstructure:"level" validate:"required"`
	} `mapstructure:"log"`
}

func Load(opts ...configx.Option) (Config, error) {
	cfg, err := configx.LoadTErr[Config](append(defaultOptions(), opts...)...)
	if err != nil {
		return Config{}, err
	}
	return finalizeConfig(cfg)
}

func finalizeConfig(cfg Config) (Config, error) {
	cfg.Agent.Environments = normalizeStringSlice(cfg.Agent.Environments)
	cfg.Discovery.Static.HCLFiles = normalizeStringSlice(cfg.Discovery.Static.HCLFiles)
	hclMonitors, err := discovery.LoadStaticMonitorsHCL(cfg.Discovery.Static.HCLFiles)
	if err != nil {
		return Config{}, fmt.Errorf("load static monitors HCL: %w", err)
	}
	cfg.Discovery.Static.Monitors = normalizeStaticMonitors(cfg.Discovery.Static.Monitor, cfg.Discovery.Static.Monitors, hclMonitors)
	return cfg, nil
}

func LoadFromFlags(flags *pflag.FlagSet, configFile string) (Config, error) {
	opts := []configx.Option{configx.WithFlagSet(flags)}
	if configFile != "" {
		if isHCLConfigFile(configFile) {
			return LoadHCLFromFlags(flags, configFile)
		}
		opts = append(opts, configx.WithFiles(configFile))
	}
	return Load(opts...)
}

func LoadHCLFromFlags(flags *pflag.FlagSet, configFile string) (Config, error) {
	base, err := loadBaseDotenvValues()
	if err != nil {
		return Config{}, err
	}
	hclValues, err := loadAgentHCLDefaults(configFile)
	if err != nil {
		return Config{}, err
	}

	cfg, err := configx.LoadTErr[Config](
		configx.WithDefaults(mergeConfigValues(base, hclValues)),
		configx.WithEnvPrefix("ORIVIS"),
		configx.WithEnvSeparator("__"),
		configx.WithPriority(configx.SourceEnv, configx.SourceArgs),
		configx.WithFlagSet(flags),
		configx.WithValidateLevel(configx.ValidateLevelStruct),
	)
	if err != nil {
		return Config{}, err
	}
	return finalizeConfig(cfg)
}

func loadBaseDotenvValues() (map[string]any, error) {
	cfg, err := configx.LoadConfig(append(defaultOptions(), configx.WithPriority(configx.SourceDotenv))...)
	if err != nil {
		return nil, fmt.Errorf("load base dotenv config: %w", err)
	}
	return cfg.All().All(), nil
}

func mergeConfigValues(base, override map[string]any) map[string]any {
	return lo.Assign(base, override)
}

func isHCLConfigFile(path string) bool {
	return strings.EqualFold(filepath.Ext(strings.TrimSpace(path)), ".hcl")
}

func defaultOptions() []configx.Option {
	return []configx.Option{
		configx.WithDefaults(map[string]any{
			"server.url":                 "http://127.0.0.1:8080",
			"agent.name":                 "local-agent",
			"agent.token":                "",
			"agent.region":               "local",
			"agent.environments":         []string{},
			"runtime":                    "host",
			"poll.interval":              30 * time.Second,
			"discovery.static.enabled":   true,
			"discovery.static.hcl_files": []string{},
			"discovery.static.monitor":   discovery.StaticMonitor{},
			"discovery.static.monitors":  []discovery.StaticMonitor{},
			"discovery.docker.enabled":   false,
			"discovery.docker.mode":      "container",
			"log.level":                  "info",
		}),
		configx.WithEnvPrefix("ORIVIS"),
		configx.WithEnvSeparator("__"),
		configx.WithValidateLevel(configx.ValidateLevelStruct),
	}
}

func normalizeStaticMonitors(single discovery.StaticMonitor, monitors, hclMonitors []discovery.StaticMonitor) []discovery.StaticMonitor {
	out := collectionlist.NewListWithCapacity[discovery.StaticMonitor](len(monitors) + len(hclMonitors) + 1)
	if hasStaticMonitor(single) {
		out.Add(single)
	}
	out.Add(monitors...)
	out.Add(hclMonitors...)
	return out.Values()
}

func hasStaticMonitor(monitor discovery.StaticMonitor) bool {
	return strings.TrimSpace(monitor.SourceKey) != "" ||
		strings.TrimSpace(monitor.Name) != "" ||
		strings.TrimSpace(monitor.Type) != "" ||
		strings.TrimSpace(monitor.Target) != "" ||
		strings.TrimSpace(monitor.EnvironmentCode) != ""
}

func normalizeStringSlice(values []string) []string {
	parts := collectionlist.FlatMapList(
		collectionlist.NewList(values...),
		func(_ int, value string) []string {
			return strings.Split(value, ",")
		},
	)
	return collectionlist.FilterMapList(parts, func(_ int, part string) (string, bool) {
		part = strings.TrimSpace(part)
		return part, part != ""
	}).Values()
}
