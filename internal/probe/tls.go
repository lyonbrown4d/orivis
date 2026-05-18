package probe

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

const (
	defaultTLSPort           = "443"
	defaultTLSDegradedBefore = 14 * 24 * time.Hour
)

type tlsProbeTarget struct {
	address        string
	serverName     string
	degradedBefore time.Duration
}

func (c *Checker) checkTLS(ctx context.Context, task protocol.AgentTask) (model.Status, map[string]any, error) {
	target, detail, err := parseTLSProbeTarget(task.Target)
	if err != nil {
		return model.StatusDown, detail, err
	}

	dialer := tls.Dialer{Config: &tls.Config{
		ServerName: target.serverName,
		MinVersion: tls.VersionTLS12,
	}}
	conn, err := dialer.DialContext(ctx, "tcp", target.address)
	if err != nil {
		return model.StatusDown, detail, wrapError(err, "execute TLS probe")
	}
	defer closeSilently(conn)

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return model.StatusDown, detail, errorf("TLS probe did not return a TLS connection")
	}

	state := tlsConn.ConnectionState()
	detail["tls_version"] = tlsVersionName(state.Version)
	if len(state.PeerCertificates) == 0 {
		return model.StatusUp, detail, nil
	}
	return tlsCertificateStatus(time.Now().UTC(), target, state.PeerCertificates[0], detail)
}

func parseTLSProbeTarget(raw string) (tlsProbeTarget, map[string]any, error) {
	target := strings.TrimSpace(raw)
	if target == "" {
		return tlsProbeTarget{}, map[string]any{"target": raw}, errorf("TLS probe target is empty")
	}
	if strings.Contains(target, "://") || strings.Contains(target, "?") {
		return parseTLSURLTarget(raw, target)
	}

	address := ensureHostPort(target, defaultTLSPort)
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return tlsProbeTarget{}, map[string]any{"target": raw}, wrapError(err, "parse TLS probe target")
	}
	return newTLSProbeTarget(address, host, defaultTLSDegradedBefore), tlsProbeDetail(address, host, defaultTLSDegradedBefore), nil
}

func parseTLSURLTarget(raw, target string) (tlsProbeTarget, map[string]any, error) {
	if !strings.Contains(target, "://") {
		target = "tls://" + target
	}

	parsed, err := url.Parse(target)
	if err != nil {
		return tlsProbeTarget{}, map[string]any{"target": raw}, wrapError(err, "parse TLS probe target")
	}
	switch strings.ToLower(parsed.Scheme) {
	case string(model.MonitorTLS), string(model.MonitorHTTP), "https":
	default:
		return tlsProbeTarget{}, map[string]any{"target": raw, "scheme": parsed.Scheme}, errorf("unsupported TLS target scheme %q", parsed.Scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		return tlsProbeTarget{}, map[string]any{"target": raw}, errorf("TLS probe target host is empty")
	}

	query := parsed.Query()
	serverName := strings.TrimSpace(query.Get("server_name"))
	if serverName == "" {
		serverName = host
	}
	degradedBefore, err := parseTLSDegradedBefore(query)
	if err != nil {
		return tlsProbeTarget{}, map[string]any{"target": raw}, err
	}

	address := net.JoinHostPort(host, firstNonEmpty(parsed.Port(), defaultTLSPort))
	return newTLSProbeTarget(address, serverName, degradedBefore), tlsProbeDetail(address, serverName, degradedBefore), nil
}

func newTLSProbeTarget(address, serverName string, degradedBefore time.Duration) tlsProbeTarget {
	return tlsProbeTarget{
		address:        address,
		serverName:     serverName,
		degradedBefore: degradedBefore,
	}
}

func parseTLSDegradedBefore(query url.Values) (time.Duration, error) {
	value := firstNonEmpty(
		query.Get("degraded_before"),
		firstNonEmpty(query.Get("degrade_before"), firstNonEmpty(query.Get("warn_before"), query.Get("warning_before"))),
	)
	if strings.TrimSpace(value) == "" {
		return defaultTLSDegradedBefore, nil
	}

	duration, err := parseHumanDuration(value)
	if err != nil {
		return 0, wrapError(err, "parse TLS degraded threshold")
	}
	if duration < 0 {
		return 0, errorf("TLS degraded threshold must be non-negative")
	}
	return duration, nil
}

func parseHumanDuration(value string) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)
	if daysValue, ok := strings.CutSuffix(trimmed, "d"); ok {
		days, err := time.ParseDuration(daysValue + "h")
		if err != nil {
			return 0, wrapError(err, "parse day duration")
		}
		return days * 24, nil
	}
	duration, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, wrapError(err, "parse duration")
	}
	return duration, nil
}

func tlsCertificateStatus(now time.Time, target tlsProbeTarget, certificate *x509.Certificate, detail map[string]any) (model.Status, map[string]any, error) {
	remaining := certificate.NotAfter.Sub(now)
	detail["not_after"] = certificate.NotAfter
	detail["expires_in_seconds"] = int64(remaining.Seconds())
	detail["degraded_before_seconds"] = int64(target.degradedBefore.Seconds())

	if remaining <= 0 {
		return model.StatusDown, detail, errorf("TLS certificate expired at %s", certificate.NotAfter.Format(time.RFC3339))
	}
	if target.degradedBefore > 0 && remaining <= target.degradedBefore {
		return model.StatusDegraded, detail, errorf("TLS certificate expires in %s", remaining.Truncate(time.Second))
	}
	return model.StatusUp, detail, nil
}

func tlsProbeDetail(address, serverName string, degradedBefore time.Duration) map[string]any {
	return map[string]any{
		"target":                  address,
		"server_name":             serverName,
		"degraded_before_seconds": int64(degradedBefore.Seconds()),
	}
}

func tlsVersionName(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return "unknown"
	}
}
