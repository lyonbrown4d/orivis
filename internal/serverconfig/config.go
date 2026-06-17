package config

import (
	"errors"
	"path"
	"strings"

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
		Addr           string `mapstructure:"addr"           validate:"required"`
		BasePath       string `mapstructure:"basepath"`
		BodyLimitBytes int    `mapstructure:"bodylimitbytes"`
	} `mapstructure:"http"`
	MDNS struct {
		Enabled  bool   `mapstructure:"enabled"`
		Service  string `mapstructure:"service"  validate:"required"`
		Domain   string `mapstructure:"domain"   validate:"required"`
		Instance string `mapstructure:"instance" validate:"required"`
		Scheme   string `mapstructure:"scheme"   validate:"required"`
		Port     int    `mapstructure:"port"`
	} `mapstructure:"mdns"`
	Log struct {
		Level string `mapstructure:"level" validate:"required"`
	} `mapstructure:"log"`
	DB struct {
		Driver       string `mapstructure:"driver"       validate:"required"`
		DSN          string `mapstructure:"dsn"`
		MaxOpenConns int    `mapstructure:"maxopenconns"`
		BusyTimeout  string `mapstructure:"busytimeout"  validate:"required"`
	} `mapstructure:"db"`
	Cache struct {
		Driver string `mapstructure:"driver" validate:"required"`
		Prefix string `mapstructure:"prefix" validate:"required"`
		Redis  struct {
			Addr     string `mapstructure:"addr"`
			Password string `mapstructure:"password"`
			DB       int    `mapstructure:"db"`
			TLS      bool   `mapstructure:"tls"`
		} `mapstructure:"redis"`
	} `mapstructure:"cache"`
	Dashboard struct {
		SnapshotTTL string `mapstructure:"snapshotttl" validate:"required"`
	} `mapstructure:"dashboard"`
	Ingest struct {
		QueueSize           int    `mapstructure:"queuesize"`
		BatchSize           int    `mapstructure:"batchsize"`
		MaxRequestBatchSize int    `mapstructure:"maxrequestbatchsize"`
		FlushInterval       string `mapstructure:"flushinterval"       validate:"required"`
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
	Notification struct {
		Webhook struct {
			Enabled         bool     `mapstructure:"enabled"`
			URL             string   `mapstructure:"url"`
			Method          string   `mapstructure:"method"          validate:"required"`
			Timeout         string   `mapstructure:"timeout"         validate:"required"`
			Cooldown        string   `mapstructure:"cooldown"        validate:"required"`
			QueueSize       int      `mapstructure:"queuesize"`
			MaxAttempts     int      `mapstructure:"maxattempts"`
			RetryInterval   string   `mapstructure:"retryinterval"   validate:"required"`
			Secret          string   `mapstructure:"secret"`
			Headers         []string `mapstructure:"headers"`
			Routes          []string `mapstructure:"routes"`
			RecoveryEnabled bool     `mapstructure:"recoveryenabled"`
		} `mapstructure:"webhook"`
	} `mapstructure:"notification"`
}

func Load(opts ...configx.Option) (Config, error) {
	cfg, err := configx.LoadTErr[Config](append(defaultOptions(), opts...)...)
	if err != nil {
		return Config{}, err
	}
	return finalizeConfig(cfg)
}

func finalizeConfig(cfg Config) (Config, error) {
	basePath, err := normalizeBasePath(cfg.HTTP.BasePath)
	if err != nil {
		return Config{}, err
	}
	cfg.HTTP.BasePath = basePath
	return cfg, nil
}

func normalizeBasePath(value string) (string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" || raw == "/" {
		return "", nil
	}
	if !strings.HasPrefix(raw, "/") {
		return "", errors.New("http base path must start with /")
	}
	if strings.ContainsAny(raw, "?#") {
		return "", errors.New("http base path must not contain query or fragment")
	}
	cleaned := path.Clean(raw)
	if cleaned == "." || cleaned == "/" {
		return "", nil
	}
	return cleaned, nil
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
		Addr           string `json:"addr"`
		BasePath       string `json:"basepath"`
		BodyLimitBytes int    `json:"bodylimitbytes"`
	} `json:"http"`
	MDNS struct {
		Enabled  bool   `json:"enabled"`
		Service  string `json:"service"`
		Domain   string `json:"domain"`
		Instance string `json:"instance"`
		Scheme   string `json:"scheme"`
		Port     int    `json:"port"`
	} `json:"mdns"`
	Log struct {
		Level string `json:"level"`
	} `json:"log"`
	DB struct {
		Driver       string `json:"driver"`
		DSN          string `json:"dsn"`
		MaxOpenConns int    `json:"maxopenconns"`
		BusyTimeout  string `json:"busytimeout"`
	} `json:"db"`
	Cache struct {
		Driver string `json:"driver"`
		Prefix string `json:"prefix"`
	} `json:"cache"`
	Dashboard struct {
		SnapshotTTL string `json:"snapshotttl"`
	} `json:"dashboard"`
	Ingest struct {
		QueueSize           int    `json:"queuesize"`
		BatchSize           int    `json:"batchsize"`
		MaxRequestBatchSize int    `json:"maxrequestbatchsize"`
		FlushInterval       string `json:"flushinterval"`
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
	Notification struct {
		Webhook struct {
			Method          string   `json:"method"`
			Timeout         string   `json:"timeout"`
			Cooldown        string   `json:"cooldown"`
			QueueSize       int      `json:"queuesize"`
			MaxAttempts     int      `json:"maxattempts"`
			RetryInterval   string   `json:"retryinterval"`
			Headers         []string `json:"headers"`
			Routes          []string `json:"routes"`
			RecoveryEnabled bool     `json:"recoveryenabled"`
		} `json:"webhook"`
	} `json:"notification"`
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
	cfg.HTTP.BasePath = ""
	cfg.HTTP.BodyLimitBytes = 4 * 1024 * 1024
	cfg.MDNS.Enabled = true
	cfg.MDNS.Service = "orivis"
	cfg.MDNS.Domain = "local."
	cfg.MDNS.Instance = "orivis-server"
	cfg.MDNS.Scheme = "http"
	cfg.MDNS.Port = 0
	cfg.Log.Level = "info"
	cfg.DB.Driver = "sqlite"
	cfg.DB.DSN = DefaultSQLiteDSN
	cfg.DB.MaxOpenConns = 4
	cfg.DB.BusyTimeout = "5s"
	cfg.Cache.Driver = "memory"
	cfg.Cache.Prefix = "orivis"
	cfg.Dashboard.SnapshotTTL = "1s"
	cfg.Ingest.QueueSize = 4096
	cfg.Ingest.BatchSize = 100
	cfg.Ingest.MaxRequestBatchSize = 1000
	cfg.Ingest.FlushInterval = "1s"
	cfg.Retention.Enabled = true
	cfg.Retention.ResultTTL = "168h"
	cfg.Retention.CleanupInterval = "1h"
	cfg.Auth.Dashboard.Username = "admin"
	cfg.Observability.Prometheus.Namespace = "orivis"
	cfg.Notification.Webhook.Method = "POST"
	cfg.Notification.Webhook.Timeout = "5s"
	cfg.Notification.Webhook.Cooldown = "5m"
	cfg.Notification.Webhook.QueueSize = 128
	cfg.Notification.Webhook.MaxAttempts = 3
	cfg.Notification.Webhook.RetryInterval = "5s"
	cfg.Notification.Webhook.Headers = []string{}
	cfg.Notification.Webhook.Routes = []string{}
	cfg.Notification.Webhook.RecoveryEnabled = true
	return cfg
}
