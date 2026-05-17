package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/arcgolabs/dbx"
	"github.com/arcgolabs/dbx/dialect"
	"github.com/arcgolabs/dbx/dialect/sqlite"
	repository "github.com/arcgolabs/dbx/repository"
	"github.com/lyonbrown4d/orivis/internal/model"
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/samber/lo"
	_ "modernc.org/sqlite" // register sqlite database driver
)

type Store struct {
	DB           *dbx.DB
	repositories *Repositories
	ids          IDGenerator
	agents       AgentStore
	monitors     MonitorStore
	results      ResultStore
}

func Open(cfg config.Config, logger *slog.Logger) (*Store, error) {
	database, err := OpenDB(cfg, logger)
	if err != nil {
		return nil, err
	}
	return New(database, NewRepositories(database), NewIDGenerator(database))
}

func OpenDB(cfg config.Config, logger *slog.Logger) (*dbx.DB, error) {
	switch normalizeDBDriver(cfg.DB.Driver) {
	case "", "sqlite", "sqlite3":
		if strings.TrimSpace(cfg.DB.Driver) == "" {
			cfg.DB.Driver = "sqlite"
		}
		if strings.TrimSpace(cfg.DB.DSN) == "" {
			cfg.DB.DSN = config.DefaultSQLiteDSN
		}
		return openSQLiteDB(cfg, logger)
	default:
		return nil, fmt.Errorf("unsupported database driver %q: supported driver is sqlite", cfg.DB.Driver)
	}
}

func New(database *dbx.DB, repositories *Repositories, ids IDGenerator) (*Store, error) {
	if database == nil {
		return nil, fmt.Errorf("%w: db is required", ErrInvalidInput)
	}
	if repositories == nil {
		repositories = NewRepositories(database)
	}
	if ids == nil {
		ids = NewIDGenerator(database)
	}

	storage := &Store{
		DB:           database,
		repositories: repositories,
		ids:          ids,
	}
	storage.agents = &agentStore{repositories: repositories, ids: ids}
	storage.monitors = &monitorStore{repositories: repositories, ids: ids}
	storage.results = &resultStore{db: database, repositories: repositories, ids: ids}
	if err := storage.Migrate(context.Background()); err != nil {
		if closeErr := database.Close(); closeErr != nil {
			return nil, fmt.Errorf("migrate sqlite store: %w; close database: %w", err, closeErr)
		}
		return nil, fmt.Errorf("migrate sqlite store: %w", err)
	}

	return storage, nil
}

func openSQLiteDB(cfg config.Config, logger *slog.Logger) (*dbx.DB, error) {
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
	configureSQLiteConnection(database, cfg.DB.DSN)
	configureSQLitePool(database, cfg)
	if err := configureSQLitePragmas(context.Background(), database, cfg); err != nil {
		if closeErr := database.Close(); closeErr != nil {
			return nil, fmt.Errorf("%w; close sqlite store: %w", err, closeErr)
		}
		return nil, err
	}

	return database, nil
}

func (s *Store) Close(context.Context) error {
	if s == nil {
		return nil
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
	if lo.Contains(environmentIDs, environmentID) {
		return environmentID, nil
	}
	return "", fmt.Errorf("%w: agent is not assigned to environment %s", ErrUnauthorized, code)
}

func (s *Store) findEnvironmentIDByCode(ctx context.Context, code string) (string, error) {
	switch {
	case s == nil:
		return "", fmt.Errorf("%w: store is not available", ErrInvalidInput)
	case s.repositories != nil:
		return findEnvironmentIDByCode(ctx, s.repositories, code)
	default:
		return "", fmt.Errorf("%w: store backend is not available", ErrInvalidInput)
	}
}

func findEnvironmentIDByCode(ctx context.Context, repositories *Repositories, code string) (string, error) {
	environment, err := repositories.environments.FirstSpec(ctx, repository.Where(environmentsSchema.Code.Eq(code)))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", fmt.Errorf("%w: environment %s", ErrNotFound, code)
		}
		return "", fmt.Errorf("find environment by code: %w", err)
	}
	return environment.ID, nil
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

func configureSQLiteConnection(database *dbx.DB, dsn string) {
	if database == nil || database.SQLDB() == nil {
		return
	}
	database.SQLDB().SetConnMaxIdleTime(5 * time.Minute)
	if isSQLiteMemoryDSN(dsn) {
		database.SQLDB().SetMaxOpenConns(1)
		database.SQLDB().SetMaxIdleConns(1)
		return
	}
	database.SQLDB().SetMaxOpenConns(4)
	database.SQLDB().SetMaxIdleConns(4)
}

func configureSQLitePool(database *dbx.DB, cfg config.Config) {
	if database == nil || database.SQLDB() == nil || isSQLiteMemoryDSN(cfg.DB.DSN) || cfg.DB.MaxOpenConns <= 0 {
		return
	}
	database.SQLDB().SetMaxOpenConns(cfg.DB.MaxOpenConns)
	database.SQLDB().SetMaxIdleConns(cfg.DB.MaxOpenConns)
}

func configureSQLitePragmas(ctx context.Context, database *dbx.DB, cfg config.Config) error {
	if database == nil {
		return nil
	}
	if err := execSQLitePragma(ctx, database, "PRAGMA foreign_keys = ON"); err != nil {
		return err
	}
	if err := execSQLitePragma(ctx, database, "PRAGMA busy_timeout = "+sqliteBusyTimeoutMillis(cfg.DB.BusyTimeout)); err != nil {
		return err
	}
	if isSQLiteMemoryDSN(cfg.DB.DSN) {
		return nil
	}
	if err := execSQLitePragma(ctx, database, "PRAGMA journal_mode = WAL"); err != nil {
		return err
	}
	return execSQLitePragma(ctx, database, "PRAGMA synchronous = NORMAL")
}

func execSQLitePragma(ctx context.Context, database *dbx.DB, query string) error {
	if _, err := database.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("configure sqlite pragma %q: %w", query, err)
	}
	return nil
}

func sqliteBusyTimeoutMillis(value string) string {
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || duration <= 0 {
		duration = 5 * time.Second
	}
	return strconv.FormatInt(duration.Milliseconds(), 10)
}

func isSQLiteMemoryDSN(dsn string) bool {
	normalizedDSN := strings.ToLower(dsn)
	return strings.Contains(normalizedDSN, "mode=memory") || strings.Contains(normalizedDSN, ":memory:")
}

func agentEnvironmentIDValues(agent model.Agent) []string {
	if agent.EnvironmentIDs == nil {
		return nil
	}
	return agent.EnvironmentIDs.Values()
}
