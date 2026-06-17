package api

import (
	"path"
	"strings"

	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
)

func httpBasePath(cfg config.Config) string {
	return strings.TrimRight(strings.TrimSpace(cfg.HTTP.BasePath), "/")
}

func prefixedPath(cfg config.Config, route string) string {
	return joinBasePath(httpBasePath(cfg), route)
}

func joinBasePath(basePath, route string) string {
	basePath = strings.TrimRight(strings.TrimSpace(basePath), "/")
	if basePath == "" {
		if route == "" {
			return "/"
		}
		if strings.HasPrefix(route, "/") {
			return route
		}
		return "/" + route
	}
	route = strings.TrimSpace(route)
	if route == "" || route == "/" {
		return basePath
	}
	return basePath + "/" + strings.TrimLeft(route, "/")
}

func staticAssetName(requestPath string) string {
	cleaned := path.Clean(strings.TrimSpace(requestPath))
	const marker = "/ui/static/"
	_, assetName, ok := strings.Cut(cleaned, marker)
	if ok {
		return strings.TrimPrefix(assetName, "/")
	}
	return strings.TrimPrefix(strings.TrimPrefix(cleaned, "/ui/static/"), "ui/static/")
}
