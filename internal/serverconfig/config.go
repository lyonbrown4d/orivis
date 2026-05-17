package config

import (
	"github.com/arcgolabs/configx"
	jsonparser "github.com/knadh/koanf/parsers/json"
	tomlparser "github.com/knadh/koanf/parsers/toml/v2"
	yamlparser "github.com/knadh/koanf/parsers/yaml"
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
	Web struct {
		Enabled bool   `mapstructure:"enabled"`
		Root    string `mapstructure:"root"    validate:"required"`
	} `mapstructure:"web"`
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

func LoadFromFlags(flags *pflag.FlagSet, configFile string, opts ...configx.Option) (Config, error) {
	loadOptions := []configx.Option{configx.WithFlagSet(flags)}
	if configFile != "" {
		loadOptions = append(loadOptions, configx.WithFiles(configFile))
	}
	loadOptions = append(loadOptions, opts...)
	return Load(loadOptions...)
}

type defaultConfigValues struct {
	App struct {
		Env string `json:"env"`
	} `json:"app"`
	HTTP struct {
		Addr string `json:"addr"`
	} `json:"http"`
	Web struct {
		Root string `json:"root"`
	} `json:"web"`
	Log struct {
		Level string `json:"level"`
	} `json:"log"`
	DB struct {
		Driver string `json:"driver"`
		DSN    string `json:"dsn"`
	} `json:"db"`
	Ingest struct {
		QueueSize     int    `json:"queuesize"`
		BatchSize     int    `json:"batchsize"`
		FlushInterval string `json:"flushinterval"`
	} `json:"ingest"`
	Retention struct {
		Enabled         bool   `json:"enabled"`
		ResultTTL       string `json:"resultttl"`
		CleanupInterval string `json:"cleanupinterval"`
	} `json:"retention"`
	Auth struct {
		Dashboard struct {
			Username string `json:"username"`
		} `json:"dashboard"`
	} `json:"auth"`
	Observability struct {
		Prometheus struct {
			Namespace string `json:"namespace"`
		} `json:"prometheus"`
	} `json:"observability"`
}

func defaultOptions() []configx.Option {
	fileParsers := configFileParserOptions()
	opts := make([]configx.Option, 0, 4+len(fileParsers))
	opts = append(opts,
		configx.WithTypedDefaults(defaultConfig()),
		configx.WithEnvPrefix("ORIVIS"),
		configx.WithEnvSeparator("__"),
		configx.WithValidateLevel(configx.ValidateLevelStruct),
	)
	return append(opts, fileParsers...)
}

func configFileParserOptions() []configx.Option {
	return []configx.Option{
		configx.WithFileParser(".json", jsonparser.Parser()),
		configx.WithFileParser(".toml", tomlparser.Parser()),
		configx.WithFileParser(".yaml", yamlparser.Parser()),
		configx.WithFileParser(".yml", yamlparser.Parser()),
	}
}

func defaultConfig() defaultConfigValues {
	var cfg defaultConfigValues
	cfg.App.Env = "development"
	cfg.HTTP.Addr = ":8080"
	cfg.Web.Root = "web/dist"
	cfg.Log.Level = "info"
	cfg.DB.Driver = "sqlite"
	cfg.DB.DSN = DefaultSQLiteDSN
	cfg.Ingest.QueueSize = 4096
	cfg.Ingest.BatchSize = 100
	cfg.Ingest.FlushInterval = "1s"
	cfg.Retention.Enabled = true
	cfg.Retention.ResultTTL = "168h"
	cfg.Retention.CleanupInterval = "1h"
	cfg.Auth.Dashboard.Username = "admin"
	cfg.Observability.Prometheus.Namespace = "orivis"
	return cfg
}
