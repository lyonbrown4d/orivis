package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
		Workers  int           `mapstructure:"workers"`
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
	cfg.Agent.Name = appendHostnameSuffix(cfg.Agent.Name)
	normalizePollConfig(&cfg)
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

func appendHostnameSuffix(value string) string {
	name := strings.TrimSpace(value)
	if name == "" {
		return name
	}

	host := strings.TrimSpace(os.Getenv("HOSTNAME"))
	if host == "" {
		hostName, err := os.Hostname()
		if err == nil {
			host = strings.TrimSpace(hostName)
		}
	}
	if host == "" {
		return name
	}

	host = strings.ToLower(host)
	if strings.HasSuffix(name, "@"+host) {
		return name
	}
	if strings.Contains(name, "@") {
		return name
	}
	return name + "@" + host
}

func LoadFromFlags(flags *pflag.FlagSet, configFile string, opts ...configx.Option) (Config, error) {
	loadOptions := []configx.Option{configx.WithFlagSet(flags)}
	if configFile != "" {
		loadOptions = append(loadOptions, configx.WithFiles(configFile))
	}
	loadOptions = append(loadOptions, opts...)
	return Load(loadOptions...)
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

func normalizeBufferConfig(cfg *Config) error {
	cfg.Buffer.Driver = strings.ToLower(strings.TrimSpace(cfg.Buffer.Driver))
	if cfg.Buffer.Driver == "" {
		cfg.Buffer.Driver = "persistent"
	}
	cfg.Buffer.Path = strings.TrimSpace(cfg.Buffer.Path)
	if cfg.Buffer.Path == "" && cfg.Buffer.Driver == "persistent" {
		cfg.Buffer.Path = filepath.Join(os.TempDir(), "orivis-agent-buffer")
	}
	switch cfg.Buffer.Driver {
	case "memory", "persistent":
		return nil
	default:
		return fmt.Errorf("unsupported buffer driver %q", cfg.Buffer.Driver)
	}
}

func normalizeDiscoveryConfig(cfg *Config) error {
	cfg.Discovery.Provider = strings.ToLower(strings.TrimSpace(cfg.Discovery.Provider))
	cfg.Discovery.Docker.Mode = strings.ToLower(strings.TrimSpace(cfg.Discovery.Docker.Mode))

	switch cfg.Discovery.Provider {
	case "docker":
		return normalizeDockerDiscovery(cfg)
	case "":
		if !cfg.Discovery.Docker.Enabled {
			return nil
		}
		if cfg.Discovery.Docker.Mode == "" {
			return errors.New("discovery provider must be set when docker discovery is enabled")
		}
		return nil
	default:
		return fmt.Errorf("unsupported discovery provider %q", cfg.Discovery.Provider)
	}
}

func normalizeDockerDiscovery(cfg *Config) error {
	cfg.Discovery.Docker.Enabled = true
	runtimeMode := strings.TrimSpace(cfg.Runtime)
	if runtimeMode == "" || strings.EqualFold(runtimeMode, "host") {
		cfg.Runtime = "docker"
	}
	if cfg.Discovery.Docker.Mode == "" {
		return errors.New("discovery docker mode is required when provider is docker")
	}
	return nil
}
