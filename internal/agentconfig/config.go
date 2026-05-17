package config

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/configx"
	"github.com/lyonbrown4d/orivis/internal/discovery"
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

func LoadFromFlags(flags *pflag.FlagSet, configFile string, opts ...configx.Option) (Config, error) {
	loadOptions := []configx.Option{configx.WithFlagSet(flags)}
	if configFile != "" {
		if isHCLConfigFile(configFile) {
			return LoadHCLFromFlags(flags, configFile, opts...)
		}
		loadOptions = append(loadOptions, configx.WithFiles(configFile))
	}
	loadOptions = append(loadOptions, opts...)
	return Load(loadOptions...)
}

func LoadHCLFromFlags(flags *pflag.FlagSet, configFile string, opts ...configx.Option) (Config, error) {
	hclValues, err := loadAgentHCLDefaults(configFile)
	if err != nil {
		return Config{}, err
	}

	loadOptions := append(defaultOptions(),
		configx.WithSource("agent-hcl", func(context.Context) (map[string]any, error) {
			return hclValues, nil
		}),
		configx.WithPriority(configx.SourceDotenv, configx.SourceCustom, configx.SourceEnv, configx.SourceArgs),
		configx.WithFlagSet(flags),
	)
	loadOptions = append(loadOptions, opts...)
	cfg, err := configx.LoadTErr[Config](loadOptions...)
	if err != nil {
		return Config{}, err
	}
	return finalizeConfig(cfg)
}

func isHCLConfigFile(path string) bool {
	return strings.EqualFold(filepath.Ext(strings.TrimSpace(path)), ".hcl")
}

type defaultConfigValues struct {
	Server struct {
		URL string `json:"url"`
	} `json:"server"`
	Agent struct {
		Name         string   `json:"name"`
		Region       string   `json:"region"`
		Environments []string `json:"environments"`
	} `json:"agent"`
	Runtime string `json:"runtime"`
	Poll    struct {
		Interval time.Duration `json:"interval"`
	} `json:"poll"`
	Discovery struct {
		Static struct {
			Enabled  bool     `json:"enabled"`
			HCLFiles []string `json:"hcl_files"`
		} `json:"static"`
		Docker struct {
			Mode string `json:"mode"`
		} `json:"docker"`
	} `json:"discovery"`
	Log struct {
		Level string `json:"level"`
	} `json:"log"`
}

func defaultOptions() []configx.Option {
	return []configx.Option{
		configx.WithTypedDefaults(defaultConfig()),
		configx.WithEnvPrefix("ORIVIS"),
		configx.WithEnvSeparator("__"),
		configx.WithValidateLevel(configx.ValidateLevelStruct),
	}
}

func defaultConfig() defaultConfigValues {
	var cfg defaultConfigValues
	cfg.Server.URL = "http://127.0.0.1:8080"
	cfg.Agent.Name = "local-agent"
	cfg.Agent.Region = "local"
	cfg.Agent.Environments = []string{}
	cfg.Runtime = "host"
	cfg.Poll.Interval = 30 * time.Second
	cfg.Discovery.Static.Enabled = true
	cfg.Discovery.Static.HCLFiles = []string{}
	cfg.Discovery.Docker.Mode = "container"
	cfg.Log.Level = "info"
	return cfg
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
