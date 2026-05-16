package config

import (
	"strings"
	"time"

	"github.com/arcgolabs/configx"
	"github.com/lyonbrown4d/orivis/internal/agent/discovery"
	"github.com/spf13/pflag"
)

type Config struct {
	Server struct {
		URL string `mapstructure:"url" validate:"required,url"`
	} `mapstructure:"server"`
	Agent struct {
		Name         string   `mapstructure:"name" validate:"required"`
		Token        string   `mapstructure:"token"`
		Region       string   `mapstructure:"region" validate:"required"`
		Environments []string `mapstructure:"environments"`
	} `mapstructure:"agent"`
	Runtime string `mapstructure:"runtime" validate:"required"`
	Poll    struct {
		Interval time.Duration `mapstructure:"interval" validate:"required"`
	} `mapstructure:"poll"`
	Discovery struct {
		Static struct {
			Enabled  bool                      `mapstructure:"enabled"`
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
	cfg.Agent.Environments = normalizeStringSlice(cfg.Agent.Environments)
	return cfg, nil
}

func LoadFromFlags(flags *pflag.FlagSet, configFile string) (Config, error) {
	opts := []configx.Option{configx.WithFlagSet(flags)}
	if configFile != "" {
		opts = append(opts, configx.WithFiles(configFile))
	}
	return Load(opts...)
}

func defaultOptions() []configx.Option {
	return []configx.Option{
		configx.WithDefaults(map[string]any{
			"server.url":                "http://127.0.0.1:8080",
			"agent.name":                "local-agent",
			"agent.token":               "",
			"agent.region":              "local",
			"agent.environments":        []string{},
			"runtime":                   "host",
			"poll.interval":             30 * time.Second,
			"discovery.static.enabled":  true,
			"discovery.static.monitors": []discovery.StaticMonitor{},
			"discovery.docker.enabled":  false,
			"discovery.docker.mode":     "container",
			"log.level":                 "info",
		}),
		configx.WithEnvPrefix("ORIVIS"),
		configx.WithValidateLevel(configx.ValidateLevelStruct),
	}
}

func normalizeStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			out = append(out, part)
		}
	}
	if out == nil {
		return []string{}
	}
	return out
}
