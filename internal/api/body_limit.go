package api

import config "github.com/lyonbrown4d/orivis/internal/serverconfig"

const defaultHTTPBodyLimitBytes = 4 * 1024 * 1024

func httpBodyLimit(cfg config.Config) int {
	if cfg.HTTP.BodyLimitBytes <= 0 {
		return defaultHTTPBodyLimitBytes
	}
	return cfg.HTTP.BodyLimitBytes
}
