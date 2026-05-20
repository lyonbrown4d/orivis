package config

import "runtime"
import "time"

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
		Workers  int           `json:"workers"`
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
	cfg.Poll.Workers = runtime.NumCPU()
	if cfg.Poll.Workers <= 0 {
		cfg.Poll.Workers = 1
	}
	cfg.Buffer.Enabled = true
	cfg.Buffer.Driver = "persistent"
	cfg.Buffer.Path = ""
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
	cfg.Log.Level = "info"
	return cfg
}
