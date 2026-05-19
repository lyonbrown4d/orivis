package config

import (
	"fmt"
	"strings"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/configx"
	jsonparser "github.com/knadh/koanf/parsers/json"
	tomlparser "github.com/knadh/koanf/parsers/toml/v2"
	yamlparser "github.com/knadh/koanf/parsers/yaml"
	"github.com/lyonbrown4d/orivis/internal/discovery"
	"github.com/spf13/pflag"
)

type Config struct {
	Server struct {
		URL  string `mapstructure:"url" validate:"omitempty,url"`
		MDNS struct {
			Service       string        `mapstructure:"service"`
			Domain        string        `mapstructure:"domain"`
			Timeout       time.Duration `mapstructure:"timeout"`
			DefaultScheme string        `mapstructure:"defaultscheme"`
		} `mapstructure:"mdns"`
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
		Jitter   time.Duration `mapstructure:"jitter"`
	} `mapstructure:"poll"`
	Buffer struct {
		Enabled  bool   `mapstructure:"enabled"`
		Driver   string `mapstructure:"driver"`
		Path     string `mapstructure:"path"`
		Capacity int    `mapstructure:"capacity" validate:"min=0"`
	} `mapstructure:"buffer"`
	Transport struct {
		RequestTimeout        time.Duration `mapstructure:"requesttimeout"`
		MaxIdleConns          int           `mapstructure:"maxidleconns"`
		MaxIdleConnsPerHost   int           `mapstructure:"maxidleconnsperhost"`
		IdleConnTimeout       time.Duration `mapstructure:"idleconntimeout"`
		TLSHandshakeTimeout   time.Duration `mapstructure:"tlshandshaketimeout"`
		ResponseHeaderTimeout time.Duration `mapstructure:"responseheadertimeout"`
		RetryAttempts         int           `mapstructure:"retryattempts"`
		RetryBaseDelay        time.Duration `mapstructure:"retrybasedelay"`
		RetryMaxDelay         time.Duration `mapstructure:"retrymaxdelay"`
		RetryJitterRatio      float64       `mapstructure:"retryjitterratio"`
		GzipResults           bool          `mapstructure:"gzipresults"`
	} `mapstructure:"transport"`
	Discovery struct {
		Provider string `mapstructure:"provider"`
		Static   struct {
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
	if err := normalizeBufferConfig(&cfg); err != nil {
		return Config{}, err
	}
	normalizeTransportConfig(&cfg)
	if err := normalizeDiscoveryConfig(&cfg); err != nil {
		return Config{}, err
	}
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
		loadOptions = append(loadOptions, configx.WithFiles(configFile))
	}
	loadOptions = append(loadOptions, opts...)
	return Load(loadOptions...)
}

type defaultConfigValues struct {
	Server struct {
		URL  string `json:"url"`
		MDNS struct {
			Service       string        `json:"service"`
			Domain        string        `json:"domain"`
			Timeout       time.Duration `json:"timeout"`
			DefaultScheme string        `json:"defaultscheme"`
		} `json:"mdns"`
	} `json:"server"`
	Agent struct {
		Name         string   `json:"name"`
		Region       string   `json:"region"`
		Environments []string `json:"environments"`
	} `json:"agent"`
	Runtime string `json:"runtime"`
	Poll    struct {
		Interval time.Duration `json:"interval"`
		Jitter   time.Duration `json:"jitter"`
	} `json:"poll"`
	Buffer struct {
		Enabled  bool   `json:"enabled"`
		Driver   string `json:"driver"`
		Path     string `json:"path"`
		Capacity int    `json:"capacity"`
	} `json:"buffer"`
	Transport struct {
		RequestTimeout        time.Duration `json:"requesttimeout"`
		MaxIdleConns          int           `json:"maxidleconns"`
		MaxIdleConnsPerHost   int           `json:"maxidleconnsperhost"`
		IdleConnTimeout       time.Duration `json:"idleconntimeout"`
		TLSHandshakeTimeout   time.Duration `json:"tlshandshaketimeout"`
		ResponseHeaderTimeout time.Duration `json:"responseheadertimeout"`
		RetryAttempts         int           `json:"retryattempts"`
		RetryBaseDelay        time.Duration `json:"retrybasedelay"`
		RetryMaxDelay         time.Duration `json:"retrymaxdelay"`
		RetryJitterRatio      float64       `json:"retryjitterratio"`
		GzipResults           bool          `json:"gzipresults"`
	} `json:"transport"`
	Discovery struct {
		Provider string `json:"provider"`
		Static   struct {
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
		configx.WithFileParser(".hcl", agentHCLFileParser()),
	}
}

func defaultConfig() defaultConfigValues {
	var cfg defaultConfigValues
	cfg.Server.URL = ""
	cfg.Server.MDNS.Service = "orivis"
	cfg.Server.MDNS.Domain = "local."
	cfg.Server.MDNS.Timeout = 5 * time.Second
	cfg.Server.MDNS.DefaultScheme = "http"
	cfg.Agent.Name = "local-agent"
	cfg.Agent.Region = "local"
	cfg.Agent.Environments = []string{}
	cfg.Runtime = "host"
	cfg.Poll.Interval = 30 * time.Second
	cfg.Poll.Jitter = 5 * time.Second
	cfg.Buffer.Enabled = true
	cfg.Buffer.Driver = "memory"
	cfg.Buffer.Path = "orivis-agent-buffer.jsonl"
	cfg.Buffer.Capacity = 1024
	cfg.Transport.RequestTimeout = 10 * time.Second
	cfg.Transport.MaxIdleConns = 100
	cfg.Transport.MaxIdleConnsPerHost = 16
	cfg.Transport.IdleConnTimeout = 90 * time.Second
	cfg.Transport.TLSHandshakeTimeout = 10 * time.Second
	cfg.Transport.ResponseHeaderTimeout = 10 * time.Second
	cfg.Transport.RetryAttempts = 3
	cfg.Transport.RetryBaseDelay = time.Second
	cfg.Transport.RetryMaxDelay = 5 * time.Second
	cfg.Transport.RetryJitterRatio = 0.2
	cfg.Transport.GzipResults = true
	cfg.Discovery.Static.Enabled = true
	cfg.Discovery.Static.HCLFiles = []string{}
	cfg.Discovery.Docker.Mode = discovery.DockerModeAuto
	cfg.Log.Level = "info"
	return cfg
}

func normalizeBufferConfig(cfg *Config) error {
	cfg.Buffer.Driver = strings.ToLower(strings.TrimSpace(cfg.Buffer.Driver))
	if cfg.Buffer.Driver == "" {
		cfg.Buffer.Driver = "memory"
	}
	cfg.Buffer.Path = strings.TrimSpace(cfg.Buffer.Path)
	if cfg.Buffer.Path == "" {
		cfg.Buffer.Path = "orivis-agent-buffer.jsonl"
	}
	switch cfg.Buffer.Driver {
	case "memory", "file":
		return nil
	default:
		return fmt.Errorf("unsupported buffer driver %q", cfg.Buffer.Driver)
	}
}

func normalizeDiscoveryConfig(cfg *Config) error {
	cfg.Discovery.Provider = strings.ToLower(strings.TrimSpace(cfg.Discovery.Provider))
	switch cfg.Discovery.Provider {
	case "":
	case "docker":
		cfg.Discovery.Docker.Enabled = true
		if runtime := strings.TrimSpace(cfg.Runtime); runtime == "" || strings.EqualFold(runtime, "host") {
			cfg.Runtime = "docker"
		}
	default:
		return fmt.Errorf("unsupported discovery provider %q", cfg.Discovery.Provider)
	}
	cfg.Discovery.Docker.Mode = strings.ToLower(strings.TrimSpace(cfg.Discovery.Docker.Mode))
	if cfg.Discovery.Docker.Mode == "" {
		cfg.Discovery.Docker.Mode = discovery.DockerModeAuto
	}
	return nil
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
