package config

import "time"

func normalizeTransportConfig(cfg *Config) {
	normalizeTransportTimeouts(cfg)
	normalizeTransportRetry(cfg)
}

func normalizeTransportTimeouts(cfg *Config) {
	if cfg.Transport.RequestTimeout <= 0 {
		cfg.Transport.RequestTimeout = 10 * time.Second
	}
	if cfg.Transport.MaxIdleConns <= 0 {
		cfg.Transport.MaxIdleConns = 100
	}
	if cfg.Transport.MaxIdleConnsPerHost <= 0 {
		cfg.Transport.MaxIdleConnsPerHost = 16
	}
	if cfg.Transport.IdleConnTimeout <= 0 {
		cfg.Transport.IdleConnTimeout = 90 * time.Second
	}
	if cfg.Transport.TLSHandshakeTimeout <= 0 {
		cfg.Transport.TLSHandshakeTimeout = 10 * time.Second
	}
	if cfg.Transport.ResponseHeaderTimeout <= 0 {
		cfg.Transport.ResponseHeaderTimeout = 10 * time.Second
	}
}

func normalizeTransportRetry(cfg *Config) {
	if cfg.Transport.RetryAttempts <= 0 {
		cfg.Transport.RetryAttempts = 1
	}
	if cfg.Transport.RetryBaseDelay <= 0 {
		cfg.Transport.RetryBaseDelay = time.Second
	}
	if cfg.Transport.RetryMaxDelay <= 0 || cfg.Transport.RetryMaxDelay < cfg.Transport.RetryBaseDelay {
		cfg.Transport.RetryMaxDelay = cfg.Transport.RetryBaseDelay
	}
	if cfg.Transport.RetryJitterRatio < 0 {
		cfg.Transport.RetryJitterRatio = 0
	}
	if cfg.Transport.RetryJitterRatio > 1 {
		cfg.Transport.RetryJitterRatio = 1
	}
}
