package config

import (
	"os"

	"github.com/arcgolabs/configx"
	"github.com/spf13/pflag"
)

type Config struct {
	App struct {
		Env string `mapstructure:"env" validate:"required"`
	} `mapstructure:"app"`
	HTTP struct {
		Addr string `mapstructure:"addr" validate:"required"`
	} `mapstructure:"http"`
	Log struct {
		Level string `mapstructure:"level" validate:"required"`
	} `mapstructure:"log"`
	DB struct {
		Driver                string `mapstructure:"driver" validate:"required"`
		DSN                   string `mapstructure:"dsn"`
		MemoryResultRetention string `mapstructure:"memory_result_retention"`
		MemoryCleanupInterval string `mapstructure:"memory_cleanup_interval"`
	} `mapstructure:"db"`
	Auth struct {
		Agent struct {
			Token string `mapstructure:"token"`
		} `mapstructure:"agent"`
		Dashboard struct {
			Enabled  bool   `mapstructure:"enabled"`
			Username string `mapstructure:"username"`
			Password string `mapstructure:"password"`
		} `mapstructure:"dashboard"`
	} `mapstructure:"auth"`
	Observability struct {
		Prometheus struct {
			Enabled   bool   `mapstructure:"enabled"`
			Namespace string `mapstructure:"namespace"`
		} `mapstructure:"prometheus"`
	} `mapstructure:"observability"`
}

func Load(opts ...configx.Option) (Config, error) {
	cfg, err := configx.LoadTErr[Config](append(defaultOptions(), opts...)...)
	if err != nil {
		return Config{}, err
	}
	applyRuntimeDefaults(&cfg)
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
			"app.env":                            "development",
			"http.addr":                          ":8080",
			"log.level":                          "info",
			"db.driver":                          "memory",
			"db.dsn":                             "",
			"db.memory_result_retention":         "24h",
			"db.memory_cleanup_interval":         "1m",
			"auth.agent.token":                   "",
			"auth.dashboard.enabled":             false,
			"auth.dashboard.username":            "admin",
			"auth.dashboard.password":            "",
			"observability.prometheus.enabled":   false,
			"observability.prometheus.namespace": "orivis",
		}),
		configx.WithEnvPrefix("ORIVIS"),
		configx.WithValidateLevel(configx.ValidateLevelStruct),
	}
}

func applyRuntimeDefaults(cfg *Config) {
	if value, ok := os.LookupEnv("ORIVIS_DB_MEMORY_RESULT_RETENTION"); ok {
		cfg.DB.MemoryResultRetention = value
	}
	if value, ok := os.LookupEnv("ORIVIS_DB_MEMORY_CLEANUP_INTERVAL"); ok {
		cfg.DB.MemoryCleanupInterval = value
	}
	if cfg.DB.MemoryResultRetention == "" {
		cfg.DB.MemoryResultRetention = "24h"
	}
	if cfg.DB.MemoryCleanupInterval == "" {
		cfg.DB.MemoryCleanupInterval = "1m"
	}
}
