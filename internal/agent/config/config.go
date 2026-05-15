package config

import (
	"time"

	"github.com/arcgolabs/configx"
	"github.com/spf13/pflag"
)

type Config struct {
	Server struct {
		URL string `mapstructure:"url" validate:"required,url"`
	} `mapstructure:"server"`
	Agent struct {
		Name   string `mapstructure:"name" validate:"required"`
		Token  string `mapstructure:"token"`
		Region string `mapstructure:"region" validate:"required"`
	} `mapstructure:"agent"`
	Runtime string `mapstructure:"runtime" validate:"required"`
	Poll    struct {
		Interval time.Duration `mapstructure:"interval" validate:"required"`
	} `mapstructure:"poll"`
	Log struct {
		Level string `mapstructure:"level" validate:"required"`
	} `mapstructure:"log"`
}

func Load(opts ...configx.Option) (Config, error) {
	cfg, err := configx.LoadTErr[Config](append(defaultOptions(), opts...)...)
	if err != nil {
		return Config{}, err
	}
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
			"server.url":    "http://127.0.0.1:8080",
			"agent.name":    "local-agent",
			"agent.token":   "",
			"agent.region":  "local",
			"runtime":       "host",
			"poll.interval": 30 * time.Second,
			"log.level":     "info",
		}),
		configx.WithEnvPrefix("ORIVIS"),
		configx.WithValidateLevel(configx.ValidateLevelStruct),
	}
}
