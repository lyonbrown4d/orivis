package probe

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/textproto"
	"net/url"
	"strings"

	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

const (
	defaultSMTPPort  = "25"
	defaultSMTPSPort = "465"
)

func (c *Checker) checkSMTP(ctx context.Context, task protocol.AgentTask) (model.Status, map[string]any, error) {
	target, serverName, secure, err := smtpProbeTarget(task.Target)
	if err != nil {
		return model.StatusDown, map[string]any{"target": task.Target}, err
	}
	conn, err := dialSMTP(ctx, target, serverName, secure)
	if err != nil {
		return model.StatusDown, map[string]any{"target": target, "tls": secure}, wrapError(err, "execute SMTP probe")
	}
	defer closeSilently(conn)
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return model.StatusDown, map[string]any{"target": target, "tls": secure}, wrapError(err, "set SMTP probe deadline")
		}
	}
	textConn := textproto.NewConn(conn)
	defer closeSilently(textConn)
	if _, _, err := textConn.ReadResponse(220); err != nil {
		return model.StatusDown, map[string]any{"target": target, "tls": secure}, wrapError(err, "read SMTP greeting")
	}
	if err := textConn.PrintfLine("NOOP"); err != nil {
		return model.StatusDown, map[string]any{"target": target, "tls": secure}, wrapError(err, "write SMTP NOOP")
	}
	if _, _, err := textConn.ReadResponse(250); err != nil {
		return model.StatusDown, map[string]any{"target": target, "tls": secure}, wrapError(err, "read SMTP NOOP response")
	}
	if err := textConn.PrintfLine("QUIT"); err != nil {
		return model.StatusDown, map[string]any{"target": target, "tls": secure}, wrapError(err, "write SMTP QUIT")
	}
	return model.StatusUp, map[string]any{"target": target, "server_name": serverName, "tls": secure}, nil
}

func dialSMTP(ctx context.Context, target, serverName string, secure bool) (net.Conn, error) {
	if secure {
		dialer := tls.Dialer{Config: &tls.Config{ServerName: serverName, MinVersion: tls.VersionTLS12}}
		conn, err := dialer.DialContext(ctx, "tcp", target)
		if err != nil {
			return nil, fmt.Errorf("dial SMTP TLS: %w", err)
		}
		return conn, nil
	}
	conn, err := new(net.Dialer).DialContext(ctx, "tcp", target)
	if err != nil {
		return nil, fmt.Errorf("dial SMTP TCP: %w", err)
	}
	return conn, nil
}

func smtpProbeTarget(rawTarget string) (string, string, bool, error) {
	target := strings.TrimSpace(rawTarget)
	if target == "" {
		return "", "", false, newError("smtp target is empty")
	}
	secure := false
	if parsed, err := url.Parse(target); err == nil && parsed.Scheme != "" {
		switch parsed.Scheme {
		case "smtp":
			target = ensureHostPort(parsed.Host, defaultSMTPPort)
		case "smtps":
			secure = true
			target = ensureHostPort(parsed.Host, defaultSMTPSPort)
		default:
			return "", "", false, errorf("unsupported SMTP probe scheme %q", parsed.Scheme)
		}
	}
	target = ensureHostPort(target, defaultSMTPPort)
	host, _, err := net.SplitHostPort(target)
	if err != nil {
		return "", "", false, wrapError(err, "parse SMTP target")
	}
	return target, host, secure, nil
}

func ensureHostPort(target, defaultPort string) string {
	if host, port, err := net.SplitHostPort(target); err == nil && host != "" && port != "" {
		return target
	}
	return net.JoinHostPort(target, defaultPort)
}
