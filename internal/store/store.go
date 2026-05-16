package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/arcgolabs/dbx"
	"github.com/arcgolabs/dbx/dialect"
	"github.com/arcgolabs/dbx/dialect/sqlite"
	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/serverconfig"
	_ "modernc.org/sqlite"
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

	db, err := dbx.Open(
		dbx.WithDriver(driver),
		dbx.WithDSN(cfg.DB.DSN),
		dbx.WithDialect(d),
		dbx.ApplyOptions(
			dbx.WithLogger(logger),
			dbx.WithDebug(cfg.App.Env != "production"),
		),
	)
	if err != nil {
		return nil, err
	}

	store := &Store{DB: db}
	store.agents = &agentStore{db: db}
	store.monitors = &monitorStore{db: db}
	store.results = &resultStore{db: db}
	if err := store.Migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Close(context.Context) error {
	if s == nil {
		return nil
	}
	if s.memory != nil {
		return s.memory.Close()
	}
	if s.DB != nil {
		return s.DB.Close()
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
	environmentIDs := agentEnvironmentIDValues(agent)
	if len(environmentIDs) == 0 {
		return "", fmt.Errorf("%w: agent has no environments", ErrInvalidInput)
	}

	code = normalizeCode(code)
	if code == "" {
		if len(environmentIDs) == 1 {
			return environmentIDs[0], nil
		}
		return "", fmt.Errorf("%w: environment code is required for multi-environment agent", ErrInvalidInput)
	}

	var environmentID string
	switch {
	case s == nil:
		return "", fmt.Errorf("%w: store is not available", ErrInvalidInput)
	case s.memory != nil:
		id, err := s.memory.memoryEnvironmentIDByCode(code)
		if err != nil {
			return "", err
		}
		environmentID = id
	case s.DB != nil:
		if err := s.DB.QueryRowContext(ctx, "SELECT id FROM environments WHERE code = ?", code).Scan(&environmentID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return "", fmt.Errorf("%w: environment %s", ErrNotFound, code)
			}
			return "", fmt.Errorf("find environment by code: %w", err)
		}
	default:
		return "", fmt.Errorf("%w: store backend is not available", ErrInvalidInput)
	}

	for _, id := range environmentIDs {
		if id == environmentID {
			return environmentID, nil
		}
	}
	return "", fmt.Errorf("%w: agent is not assigned to environment %s", ErrUnauthorized, code)
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
