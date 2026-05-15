package store

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/arcgolabs/dbx"
	"github.com/arcgolabs/dbx/dialect"
	"github.com/arcgolabs/dbx/dialect/sqlite"
	"github.com/lyonbrown4d/orivis/internal/server/config"
	_ "modernc.org/sqlite"
)

type Store struct {
	DB       *dbx.DB
	agents   AgentStore
	monitors MonitorStore
	results  ResultStore
}

func Open(cfg config.Config, logger *slog.Logger) (*Store, error) {
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
	if s == nil || s.DB == nil {
		return nil
	}
	return s.DB.Close()
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

func resolveDialect(driver string) (dialect.Dialect, string, error) {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "", "sqlite", "sqlite3":
		return sqlite.New(), "sqlite", nil
	default:
		return nil, "", fmt.Errorf("unsupported database driver %q: dbx skeleton currently wires sqlite", driver)
	}
}
