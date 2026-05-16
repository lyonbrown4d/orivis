package config

import (
	"github.com/arcgolabs/configx"
	"github.com/spf13/pflag"
)

const DefaultSQLiteDSN = "file:orivis?mode=memory&cache=shared"

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
		DSN    string `mapstructure:"dsn"`
	} `mapstructure:"db"`
	Ingest struct {
		QueueSize     int    `mapstructure:"queuesize"`
		BatchSize     int    `mapstructure:"batchsize"`
		FlushInterval string `mapstructure:"flushinterval" validate:"required"`
	} `mapstructure:"ingest"`
	Retention struct {
		Enabled         bool   `mapstructure:"enabled"`
		ResultTTL       string `mapstructure:"resultttl"       validate:"required"`
		CleanupInterval string `mapstructure:"cleanupinterval" validate:"required"`
	} `mapstructure:"retention"`
	Auth struct {
		Agent struct {
			Token string `mapstructure:"token"`
		} `mapstructure:"agent"`
		Dashboard struct {
			Enabled      bool   `mapstructure:"enabled"`
			Username     string `mapstructure:"username"`
			Password     string `mapstructure:"password"`
			JWTSecret    string `mapstructure:"jwt_secret"`
			SecureCookie bool   `mapstructure:"secure_cookie"`
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
			"db.dsn":                             DefaultSQLiteDSN,
			"ingest.queuesize":                   4096,
			"ingest.batchsize":                   100,
			"ingest.flushinterval":               "1s",
			"retention.enabled":                  true,
			"retention.resultttl":                "168h",
			"retention.cleanupinterval":          "1h",
			"auth.agent.token":                   "",
			"auth.dashboard.enabled":             false,
			"auth.dashboard.username":            "admin",
			"auth.dashboard.password":            "",
			"auth.dashboard.jwt_secret":          "",
			"auth.dashboard.secure_cookie":       false,
			"observability.prometheus.enabled":   false,
			"observability.prometheus.namespace": "orivis",
		}),
		configx.WithEnvPrefix("ORIVIS"),
		configx.WithEnvSeparator("__"),
		configx.WithValidateLevel(configx.ValidateLevelStruct),
	}
}
