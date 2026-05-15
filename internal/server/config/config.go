package config

import (
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
		Driver string `mapstructure:"driver" validate:"required"`
		DSN    string `mapstructure:"dsn" validate:"required"`
	} `mapstructure:"db"`
	Auth struct {
		Agent struct {
			Token string `mapstructure:"token"`
		} `mapstructure:"agent"`
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
			"db.driver":                          "sqlite",
			"db.dsn":                             "file:orivis.db",
			"auth.agent.token":                   "",
			"observability.prometheus.enabled":   false,
			"observability.prometheus.namespace": "orivis",
		}),
		configx.WithEnvPrefix("ORIVIS"),
		configx.WithValidateLevel(configx.ValidateLevelStruct),
	}
}
