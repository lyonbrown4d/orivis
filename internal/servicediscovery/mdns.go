// Package servicediscovery provides local service discovery helpers.
package servicediscovery

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/mdns"
)

const (
	DefaultMDNSService = "orivis"
	DefaultMDNSDomain  = "local."
	DefaultMDNSScheme  = "http"
	DefaultMDNSTimeout = 5 * time.Second
)

type MDNSAdvertiseConfig struct {
	Enabled  bool
	Service  string
	Domain   string
	Instance string
	Scheme   string
	Port     int
	BasePath string
	Version  string
}

type MDNSResolveConfig struct {
	Service       string
	Domain        string
	Timeout       time.Duration
	DefaultScheme string
}

type ServerEndpoint struct {
	URL    string
	Source string
}

type MDNSAdvertiser struct {
	cfg    MDNSAdvertiseConfig
	logger *slog.Logger
	server *mdns.Server
}

func NewMDNSAdvertiser(cfg MDNSAdvertiseConfig, logger *slog.Logger) *MDNSAdvertiser {
	return &MDNSAdvertiser{
		cfg:    normalizeAdvertiseConfig(cfg),
		logger: logger,
	}
}

func (a *MDNSAdvertiser) Start(context.Context) error {
	if a == nil || !a.cfg.Enabled {
		return nil
	}
	if a.cfg.Port <= 0 {
		return newError("mDNS advertise port is required")
	}
	text := []string{
		"role=server",
		"scheme=" + a.cfg.Scheme,
		"version=" + a.cfg.Version,
	}
	if a.cfg.BasePath != "" {
		text = append(text, "path="+a.cfg.BasePath)
	}
	service, err := mdns.NewMDNSService(a.cfg.Instance, normalizeService(a.cfg.Service), normalizeDomain(a.cfg.Domain), "", a.cfg.Port, nil, text)
	if err != nil {
		return wrapError(err, "create mDNS service")
	}
	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		return wrapError(err, "start mDNS server")
	}
	a.server = server
	if a.logger != nil {
		a.logger.Info("mDNS server discovery advertised", "service", normalizeService(a.cfg.Service), "domain", normalizeDomain(a.cfg.Domain), "instance", a.cfg.Instance, "scheme", a.cfg.Scheme, "port", a.cfg.Port, "base_path", a.cfg.BasePath)
	}
	return nil
}

func (a *MDNSAdvertiser) Stop(context.Context) error {
	if a == nil || a.server == nil {
		return nil
	}
	if err := a.server.Shutdown(); err != nil {
		return wrapError(err, "stop mDNS server")
	}
	if a.logger != nil {
		a.logger.Info("mDNS server discovery stopped", "service", normalizeService(a.cfg.Service), "instance", a.cfg.Instance)
	}
	return nil
}

func ResolveMDNSServer(ctx context.Context, cfg MDNSResolveConfig, logger *slog.Logger) (ServerEndpoint, error) {
	cfg = normalizeResolveConfig(cfg)
	resolveCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	entries := make(chan *mdns.ServiceEntry, 8)
	errCh := make(chan error, 1)
	go func() {
		errCh <- mdns.QueryContext(resolveCtx, &mdns.QueryParam{
			Service: normalizeService(cfg.Service),
			Domain:  normalizeDomain(cfg.Domain),
			Timeout: cfg.Timeout,
			Entries: entries,
		})
	}()

	if logger != nil {
		logger.Info("resolving server URL with mDNS", "service", normalizeService(cfg.Service), "domain", normalizeDomain(cfg.Domain), "timeout", cfg.Timeout)
	}
	return waitForMDNSServer(resolveCtx, entries, errCh, cfg.DefaultScheme, logger)
}

func waitForMDNSServer(ctx context.Context, entries <-chan *mdns.ServiceEntry, errCh <-chan error, defaultScheme string, logger *slog.Logger) (ServerEndpoint, error) {
	for {
		endpoint, ok, err := receiveMDNSEvent(ctx, entries, errCh, defaultScheme, logger)
		if err != nil {
			return ServerEndpoint{}, err
		}
		if ok {
			return endpoint, nil
		}
	}
}

func receiveMDNSEvent(ctx context.Context, entries <-chan *mdns.ServiceEntry, errCh <-chan error, defaultScheme string, logger *slog.Logger) (ServerEndpoint, bool, error) {
	select {
	case entry, ok := <-entries:
		return serverEndpointFromMDNSEntry(entry, ok, defaultScheme, logger)
	case err := <-errCh:
		return ServerEndpoint{}, false, wrapMDNSQueryError(err)
	case <-ctx.Done():
		return ServerEndpoint{}, false, wrapError(ctx.Err(), "resolve mDNS server")
	}
}

func serverEndpointFromMDNSEntry(entry *mdns.ServiceEntry, channelOpen bool, defaultScheme string, logger *slog.Logger) (ServerEndpoint, bool, error) {
	if !channelOpen {
		return ServerEndpoint{}, false, newError("mDNS entries closed before server was discovered")
	}
	endpoint, ok := endpointFromEntry(entry, defaultScheme)
	if !ok {
		return ServerEndpoint{}, false, nil
	}
	if logger != nil {
		logger.Info("server URL resolved with mDNS", "url", endpoint.URL, "instance", entry.Name)
	}
	return endpoint, true, nil
}

func wrapMDNSQueryError(err error) error {
	if err == nil || errors.Is(err, context.Canceled) {
		return nil
	}
	return wrapError(err, "browse mDNS service")
}

func HTTPPortFromAddr(addr string, fallback int) int {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fallback
	}
	parsed, err := strconv.Atoi(port)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func normalizeAdvertiseConfig(cfg MDNSAdvertiseConfig) MDNSAdvertiseConfig {
	cfg.Service = normalizeServiceName(cfg.Service)
	cfg.Domain = normalizeDomain(cfg.Domain)
	cfg.Instance = strings.TrimSpace(cfg.Instance)
	if cfg.Instance == "" {
		cfg.Instance = "orivis-server"
	}
	cfg.Scheme = strings.ToLower(strings.TrimSpace(cfg.Scheme))
	if cfg.Scheme == "" {
		cfg.Scheme = DefaultMDNSScheme
	}
	cfg.BasePath = normalizeBasePath(cfg.BasePath)
	return cfg
}

func normalizeResolveConfig(cfg MDNSResolveConfig) MDNSResolveConfig {
	cfg.Service = normalizeServiceName(cfg.Service)
	cfg.Domain = normalizeDomain(cfg.Domain)
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultMDNSTimeout
	}
	cfg.DefaultScheme = strings.ToLower(strings.TrimSpace(cfg.DefaultScheme))
	if cfg.DefaultScheme == "" {
		cfg.DefaultScheme = DefaultMDNSScheme
	}
	return cfg
}

func normalizeServiceName(service string) string {
	service = strings.TrimSpace(service)
	if service == "" {
		return DefaultMDNSService
	}
	return strings.TrimPrefix(strings.TrimSuffix(service, "."), "_")
}

func normalizeService(service string) string {
	service = normalizeServiceName(service)
	if strings.Contains(service, "._tcp") {
		return service
	}
	return "_" + service + "._tcp"
}

func normalizeDomain(domain string) string {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return DefaultMDNSDomain
	}
	if !strings.HasSuffix(domain, ".") {
		return domain + "."
	}
	return domain
}

func endpointFromEntry(entry *mdns.ServiceEntry, defaultScheme string) (ServerEndpoint, bool) {
	if entry == nil || !isServerEntry(entry) || entry.Port <= 0 {
		return ServerEndpoint{}, false
	}
	host, ok := entryHost(entry)
	if !ok {
		return ServerEndpoint{}, false
	}
	scheme := firstTextValue(entry.InfoFields, "scheme")
	if scheme == "" {
		scheme = defaultScheme
	}
	basePath := normalizeBasePath(firstTextValue(entry.InfoFields, "path"))
	return ServerEndpoint{
		URL:    fmt.Sprintf("%s://%s:%d%s", scheme, host, entry.Port, basePath),
		Source: "mdns",
	}, true
}

func normalizeBasePath(basePath string) string {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" || basePath == "/" {
		return ""
	}
	basePath, _, _ = strings.Cut(basePath, "?")
	basePath, _, _ = strings.Cut(basePath, "#")
	basePath = path.Clean("/" + strings.Trim(basePath, "/"))
	if basePath == "." || basePath == "/" {
		return ""
	}
	return basePath
}

func isServerEntry(entry *mdns.ServiceEntry) bool {
	role := firstTextValue(entry.InfoFields, "role")
	return role == "" || strings.EqualFold(role, "server")
}

func entryHost(entry *mdns.ServiceEntry) (string, bool) {
	if entry.AddrV4 != nil {
		return entry.AddrV4.String(), true
	}
	if entry.AddrV6IPAddr != nil && entry.AddrV6IPAddr.IP != nil {
		return "[" + entry.AddrV6IPAddr.IP.String() + "]", true
	}
	host := strings.TrimSuffix(strings.TrimSpace(entry.Host), ".")
	return host, host != ""
}

func firstTextValue(text []string, key string) string {
	prefix := key + "="
	for _, item := range text {
		if value, ok := strings.CutPrefix(item, prefix); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
