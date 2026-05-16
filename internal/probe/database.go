package probe

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	mysql "github.com/go-sql-driver/mysql"

	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/protocol"
	// Register the pgx database/sql driver used by the database probe.
	_ "github.com/jackc/pgx/v5/stdlib"
	// Register the sqlite database/sql driver used by the database probe.
	_ "modernc.org/sqlite"
)

const defaultMySQLPort = "3306"

func (c *Checker) checkDatabase(ctx context.Context, task protocol.AgentTask) (model.Status, map[string]any, error) {
	driver, dsn, err := databaseProbeTarget(task.Type, task.Target)
	if err != nil {
		return model.StatusDown, map[string]any{"target": task.Target}, err
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return model.StatusDown, map[string]any{"target": task.Target, "driver": driver}, fmt.Errorf("open database probe target: %w", err)
	}
	defer closeSilently(db)
	if pingErr := db.PingContext(ctx); pingErr != nil {
		return model.StatusDown, map[string]any{"target": task.Target, "driver": driver}, fmt.Errorf("execute database probe: %w", pingErr)
	}
	return model.StatusUp, map[string]any{"target": task.Target, "driver": driver}, nil
}

func databaseProbeTarget(monitorType, target string) (string, string, error) {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return "", "", errors.New("database target is empty")
	}
	lowerTarget := strings.ToLower(trimmed)
	lowerType := strings.ToLower(strings.TrimSpace(monitorType))
	switch {
	case isSQLiteType(lowerType):
		return "sqlite", trimDatabasePrefix(trimmed), nil
	case isMySQLType(lowerType):
		return mysqlProbeTarget(trimmed)
	case isPostgresType(lowerType):
		return "pgx", trimmed, nil
	default:
		return databaseProbeTargetByPrefix(lowerTarget, trimmed, target)
	}
}

func databaseProbeTargetByPrefix(lowerTarget, trimmed, target string) (string, string, error) {
	switch {
	case strings.HasPrefix(lowerTarget, "sqlite://"):
		return "sqlite", trimmed[len("sqlite://"):], nil
	case strings.HasPrefix(lowerTarget, "sqlite3://"):
		return "sqlite", trimmed[len("sqlite3://"):], nil
	case strings.HasPrefix(lowerTarget, "mysql://"):
		return mysqlProbeTarget(trimmed)
	case strings.HasPrefix(lowerTarget, "postgres://"), strings.HasPrefix(lowerTarget, "postgresql://"):
		return "pgx", trimmed, nil
	case strings.HasPrefix(lowerTarget, "file:"), trimmed == ":memory:":
		return "sqlite", trimmed, nil
	default:
		return "", "", fmt.Errorf("unsupported database probe target %q", target)
	}
}

func isSQLiteType(monitorType string) bool {
	return monitorType == string(model.MonitorSQLite)
}

func isMySQLType(monitorType string) bool {
	return monitorType == string(model.MonitorMySQL)
}

func isPostgresType(monitorType string) bool {
	return monitorType == string(model.MonitorPostgres) || monitorType == "postgresql" || monitorType == "pg"
}

func mysqlProbeTarget(target string) (string, string, error) {
	if !strings.HasPrefix(strings.ToLower(target), "mysql://") {
		return "mysql", target, nil
	}
	dsn, err := mysqlURLDSN(target)
	if err != nil {
		return "", "", err
	}
	return "mysql", dsn, nil
}

func mysqlURLDSN(target string) (string, error) {
	parsed, err := url.Parse(target)
	if err != nil {
		return "", fmt.Errorf("parse MySQL URL: %w", err)
	}
	host := parsed.Hostname()
	if host == "" {
		return "", errors.New("mysql URL host is empty")
	}
	password, _ := parsed.User.Password()
	config := mysql.Config{
		User:   parsed.User.Username(),
		Passwd: password,
		Net:    "tcp",
		Addr:   mysqlAddress(parsed),
		DBName: strings.TrimPrefix(parsed.Path, "/"),
		Params: firstQueryValues(parsed.Query()),
	}
	return config.FormatDSN(), nil
}

func mysqlAddress(parsed *url.URL) string {
	port := parsed.Port()
	if port == "" {
		port = defaultMySQLPort
	}
	return net.JoinHostPort(parsed.Hostname(), port)
}

func firstQueryValues(values url.Values) map[string]string {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]string, len(values))
	for key, items := range values {
		if len(items) > 0 {
			result[key] = items[0]
		}
	}
	return result
}

func trimDatabasePrefix(target string) string {
	lowerTarget := strings.ToLower(target)
	switch {
	case strings.HasPrefix(lowerTarget, "sqlite://"):
		return target[len("sqlite://"):]
	case strings.HasPrefix(lowerTarget, "sqlite3://"):
		return target[len("sqlite3://"):]
	default:
		return target
	}
}
