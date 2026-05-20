package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/arcgolabs/dbx"
	repository "github.com/arcgolabs/dbx/repository"
	"github.com/lyonbrown4d/orivis/internal/model"
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/samber/lo"
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
	storage.monitors = &monitorStore{repositories: repositories, ids: ids, db: database}
	storage.results = &resultStore{db: database, repositories: repositories, ids: ids}
	if err := storage.Migrate(context.Background()); err != nil {
		if closeErr := database.Close(); closeErr != nil {
			return nil, fmt.Errorf("migrate store: %w; close database: %w", err, closeErr)
		}
		return nil, fmt.Errorf("migrate store: %w", err)
	}

	return storage, nil
}


func (s *Store) Close(context.Context) error {
	if s == nil {
		return nil
	}
	if s.DB != nil {
		if err := s.DB.Close(); err != nil {
			return fmt.Errorf("close database store: %w", err)
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

func agentEnvironmentIDValues(agent model.Agent) []string {
	if agent.EnvironmentIDs == nil {
		return nil
	}
	return agent.EnvironmentIDs.Values()
}
