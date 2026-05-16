package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/arcgolabs/dbx"
	"github.com/arcgolabs/dbx/dialect"
	"github.com/arcgolabs/dbx/dialect/sqlite"
	"github.com/lyonbrown4d/orivis/internal/model"
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	_ "modernc.org/sqlite" // register sqlite database driver
)

type Store struct {
	DB       *dbx.DB
	memory   *memoryStore
	agents   AgentStore
	monitors MonitorStore
	results  ResultStore
}

func Open(cfg config.Config, logger *slog.Logger) (*Store, error) {
	switch normalizeDBDriver(cfg.DB.Driver) {
	case "", "memory", "mem", "inmemory":
		return openMemoryStore(cfg.DB.ResultRetention, cfg.DB.CleanupInterval, logger)
	case "sqlite", "sqlite3":
		return openSQLiteStore(cfg, logger)
	default:
		return nil, fmt.Errorf("unsupported database driver %q: supported drivers are memory and sqlite", cfg.DB.Driver)
	}
}

func openSQLiteStore(cfg config.Config, logger *slog.Logger) (*Store, error) {
	if strings.TrimSpace(cfg.DB.DSN) == "" {
		return nil, fmt.Errorf("%w: sqlite db.dsn is required", ErrInvalidInput)
	}

	d, driver, err := resolveDialect(cfg.DB.Driver)
	if err != nil {
		return nil, err
	}

	database, err := dbx.Open(
		dbx.WithDriver(driver),
		dbx.WithDSN(cfg.DB.DSN),
		dbx.WithDialect(d),
		dbx.ApplyOptions(
			dbx.WithLogger(logger),
			dbx.WithDebug(cfg.App.Env != "production"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("open sqlite store: %w", err)
	}

	storage := &Store{DB: database}
	storage.agents = &agentStore{db: database}
	storage.monitors = &monitorStore{db: database}
	storage.results = &resultStore{db: database}
	if err := storage.Migrate(context.Background()); err != nil {
		if closeErr := database.Close(); closeErr != nil {
			return nil, fmt.Errorf("migrate sqlite store: %w; close database: %w", err, closeErr)
		}
		return nil, fmt.Errorf("migrate sqlite store: %w", err)
	}

	return storage, nil
}

func (s *Store) Close(context.Context) error {
	if s == nil {
		return nil
	}
	if s.memory != nil {
		return s.memory.Close()
	}
	if s.DB != nil {
		if err := s.DB.Close(); err != nil {
			return fmt.Errorf("close sqlite store: %w", err)
		}
	}
	return nil
}

func (s *Store) AgentStore() AgentStore {
	if s == nil {
		return nil
	}
	return s.agents
}

func (s *Store) MonitorStore() MonitorStore {
	if s == nil {
		return nil
	}
	return s.monitors
}

func (s *Store) ResultStore() ResultStore {
	if s == nil {
		return nil
	}
	return s.results
}

func (s *Store) EnvironmentIDForAgent(ctx context.Context, agent model.Agent, code string) (string, error) {
	environmentIDs, err := requiredAgentEnvironmentIDs(agent)
	if err != nil {
		return "", err
	}
	code = normalizeCode(code)
	if code == "" {
		return defaultAgentEnvironmentID(environmentIDs)
	}

	environmentID, err := s.findEnvironmentIDByCode(ctx, code)
	if err != nil {
		return "", err
	}
	if slices.Contains(environmentIDs, environmentID) {
		return environmentID, nil
	}
	return "", fmt.Errorf("%w: agent is not assigned to environment %s", ErrUnauthorized, code)
}

func (s *Store) findEnvironmentIDByCode(ctx context.Context, code string) (string, error) {
	switch {
	case s == nil:
		return "", fmt.Errorf("%w: store is not available", ErrInvalidInput)
	case s.memory != nil:
		return s.memory.memoryEnvironmentIDByCode(code)
	case s.DB != nil:
		return findSQLiteEnvironmentIDByCode(ctx, s.DB, code)
	default:
		return "", fmt.Errorf("%w: store backend is not available", ErrInvalidInput)
	}
}

func findSQLiteEnvironmentIDByCode(ctx context.Context, database *dbx.DB, code string) (string, error) {
	var environmentID string
	if err := database.QueryRowContext(ctx, "SELECT id FROM environments WHERE code = ?", code).Scan(&environmentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("%w: environment %s", ErrNotFound, code)
		}
		return "", fmt.Errorf("find environment by code: %w", err)
	}
	return environmentID, nil
}

func requiredAgentEnvironmentIDs(agent model.Agent) ([]string, error) {
	environmentIDs := agentEnvironmentIDValues(agent)
	if len(environmentIDs) == 0 {
		return nil, fmt.Errorf("%w: agent has no environments", ErrInvalidInput)
	}
	return environmentIDs, nil
}

func defaultAgentEnvironmentID(environmentIDs []string) (string, error) {
	if len(environmentIDs) == 1 {
		return environmentIDs[0], nil
	}
	return "", fmt.Errorf("%w: environment code is required for multi-environment agent", ErrInvalidInput)
}

func resolveDialect(driver string) (dialect.Dialect, string, error) {
	switch normalizeDBDriver(driver) {
	case "sqlite", "sqlite3":
		return sqlite.New(), "sqlite", nil
	default:
		return nil, "", fmt.Errorf("unsupported database driver %q: sqlite dialect is required", driver)
	}
}

func normalizeDBDriver(driver string) string {
	return strings.ToLower(strings.TrimSpace(driver))
}

func agentEnvironmentIDValues(agent model.Agent) []string {
	if agent.EnvironmentIDs == nil {
		return nil
	}
	return agent.EnvironmentIDs.Values()
}

func closeRows(rows *sql.Rows) {
	if rows == nil {
		return
	}
	if err := rows.Close(); err != nil {
		return
	}
}
