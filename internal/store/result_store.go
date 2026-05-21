package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/dbx"
	repository "github.com/arcgolabs/dbx/repository"
	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/samber/lo"
	"github.com/samber/oops"
)

type ResultStore interface {
	Record(ctx context.Context, params RecordProbeResultParams) (model.ProbeResult, error)
	RecordBatch(ctx context.Context, params []RecordProbeResultParams) ([]model.ProbeResult, error)
	DeleteBefore(ctx context.Context, before time.Time) (int64, error)
}

type RecordProbeResultParams struct {
	Agent        model.Agent
	ResultID     string
	MonitorID    string
	Status       model.Status
	Latency      time.Duration
	ErrorMessage string
	CheckedAt    time.Time
	RawDetail    []byte
}

type resultStore struct {
	db           *dbx.DB
	repositories *Repositories
	ids          IDGenerator
}

type resultQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func (s *resultStore) Record(ctx context.Context, params RecordProbeResultParams) (model.ProbeResult, error) {
	results, err := s.RecordBatch(ctx, []RecordProbeResultParams{params})
	if err != nil {
		return model.ProbeResult{}, err
	}
	if len(results) == 0 {
		return model.ProbeResult{}, fmt.Errorf("%w: result batch returned no records", ErrInvalidInput)
	}
	return results[0], nil
}

func (s *resultStore) RecordBatch(ctx context.Context, params []RecordProbeResultParams) ([]model.ProbeResult, error) {
	if len(params) == 0 {
		return nil, nil
	}

	var results []model.ProbeResult
	err := s.repositories.probeResults.InTx(ctx, nil, func(tx *dbx.Tx, repo *repository.Base[probeResultRow, probeResultSchema]) error {
		nextResults, rows, err := s.prepareProbeResultRows(ctx, tx, repo, params)
		if err != nil {
			return err
		}
		if err := repo.CreateMany(ctx, rows...); err != nil {
			return fmt.Errorf("create probe result batch: %w", err)
		}
		results = nextResults
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("record probe result batch: %w", err)
	}

	return results, nil
}

func (s *resultStore) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("%w", oops.New("result store is not available"))
	}
	result, err := s.db.ExecContext(ctx, "DELETE FROM probe_results WHERE checked_at < ?", before.UTC())
	if err != nil {
		return 0, fmt.Errorf("%w", oops.Wrapf(err, "%s", "delete old probe results"))
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("%w", oops.Wrapf(err, "%s", "read deleted probe result count"))
	}
	return rows, nil
}

func (s *resultStore) prepareProbeResultRows(
	ctx context.Context,
	queryer resultQueryer,
	repo *repository.Base[probeResultRow, probeResultSchema],
	params []RecordProbeResultParams,
) ([]model.ProbeResult, []*probeResultRow, error) {
	normalized, err := normalizeProbeResultParamList(params)
	if err != nil {
		return nil, nil, err
	}
	existing, err := existingProbeResultsByResultID(ctx, repo, normalized)
	if err != nil {
		return nil, nil, err
	}
	existingResults := orderedExistingProbeResults(normalized, existing)
	pending := pendingProbeResults(normalized, existing)
	monitors, err := s.monitorLookupForAgentBatch(ctx, queryer, pending.Values())
	if err != nil {
		return nil, nil, err
	}
	prepared, err := collectionlist.ReduceErrList(
		pending,
		preparedProbeResultRows{
			results: collectionlist.NewList(existingResults...),
			rows:    collectionlist.NewListWithCapacity[*probeResultRow](pending.Len()),
		},
		func(out preparedProbeResultRows, _ int, params normalizedProbeResultParams) (preparedProbeResultRows, error) {
			monitor, ok := monitors[monitorAgentKey(params.MonitorID, params.Agent.ID)]
			if !ok {
				return out, fmt.Errorf("%w: assigned monitor %s", ErrNotFound, params.MonitorID)
			}
			result, row, rowErr := s.prepareProbeResultRowWithMonitor(ctx, params, monitor)
			if rowErr != nil {
				return out, rowErr
			}
			out.results.Add(result)
			out.rows.Add(row)
			return out, nil
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("prepare probe result rows: %w", err)
	}
	return prepared.results.Values(), prepared.rows.Values(), nil
}

type preparedProbeResultRows struct {
	results *collectionlist.List[model.ProbeResult]
	rows    *collectionlist.List[*probeResultRow]
}

func (s *resultStore) prepareProbeResultRowWithMonitor(
	ctx context.Context,
	normalized normalizedProbeResultParams,
	monitor model.Monitor,
) (model.ProbeResult, *probeResultRow, error) {
	id, err := s.ids.NewID(ctx, "res")
	if err != nil {
		return model.ProbeResult{}, nil, fmt.Errorf("generate probe result id: %w", err)
	}
	now := time.Now().UTC()
	row := newProbeResultRow(id, normalized, monitor, now)

	return model.ProbeResult{
		ID:            id,
		ResultID:      normalized.ResultID,
		MonitorID:     monitor.ID,
		AgentID:       normalized.Agent.ID,
		RegionID:      normalized.Agent.RegionID,
		EnvironmentID: monitor.EnvironmentID,
		Status:        normalized.Status,
		Latency:       normalized.Latency,
		ErrorMessage:  normalized.ErrorMessage,
		CheckedAt:     normalized.CheckedAt,
		RawDetail:     normalized.RawDetail,
		CreatedAt:     now,
	}, row, nil
}

type normalizedProbeResultParams struct {
	Agent        model.Agent
	ResultID     string
	MonitorID    string
	Status       model.Status
	Latency      time.Duration
	ErrorMessage string
	CheckedAt    time.Time
	RawDetail    []byte
}

func normalizeProbeResultParams(params RecordProbeResultParams) (normalizedProbeResultParams, error) {
	out := normalizedProbeResultParams{
		Agent:        params.Agent,
		ResultID:     strings.TrimSpace(params.ResultID),
		MonitorID:    strings.TrimSpace(params.MonitorID),
		Status:       params.Status,
		Latency:      params.Latency,
		ErrorMessage: strings.TrimSpace(params.ErrorMessage),
		CheckedAt:    params.CheckedAt.UTC(),
		RawDetail:    params.RawDetail,
	}
	if out.CheckedAt.IsZero() {
		out.CheckedAt = time.Now().UTC()
	}
	if out.Latency < 0 {
		out.Latency = 0
	}

	switch {
	case out.Agent.ID == "":
		return out, fmt.Errorf("%w: agent is required", ErrInvalidInput)
	case out.MonitorID == "":
		return out, fmt.Errorf("%w: monitor id is required", ErrInvalidInput)
	case !validProbeStatus(out.Status):
		return out, fmt.Errorf("%w: invalid probe status %q", ErrInvalidInput, out.Status)
	default:
		return out, nil
	}
}

func validProbeStatus(status model.Status) bool {
	return lo.Contains([]model.Status{model.StatusUp, model.StatusDown, model.StatusDegraded, model.StatusUnknown}, status)
}
