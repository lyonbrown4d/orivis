package store

import (
	"context"
	"fmt"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/dbx/querydsl"
	repository "github.com/arcgolabs/dbx/repository"
	"github.com/lyonbrown4d/orivis/internal/model"
)

func existingProbeResultsByResultID(
	ctx context.Context,
	repo *repository.Base[probeResultRow, probeResultSchema],
	params *collectionlist.List[normalizedProbeResultParams],
) (map[string]model.ProbeResult, error) {
	resultIDs := uniqueResultIDs(params)
	if resultIDs.Len() == 0 {
		return map[string]model.ProbeResult{}, nil
	}

	schema := probeResultSchemaResource()
	rows, err := repo.List(
		ctx,
		querydsl.Select(querydsl.AllColumns(schema).Values()...).
			From(schema).
			Where(schema.ResultID.InList(resultIDs)),
	)
	if err != nil {
		return nil, fmt.Errorf("find existing probe results: %w", err)
	}

	values := rows.Values()
	out := make(map[string]model.ProbeResult, rows.Len())
	for index := range values {
		row := values[index]
		result, err := row.model()
		if err != nil {
			return nil, fmt.Errorf("map existing probe result: %w", err)
		}
		out[row.ResultID] = result
	}
	return out, nil
}

func uniqueResultIDs(params *collectionlist.List[normalizedProbeResultParams]) *collectionlist.List[string] {
	values := params.Values()
	seen := make(map[string]struct{}, len(values))
	out := collectionlist.NewListWithCapacity[string](len(values))
	for index := range values {
		params := values[index]
		if params.ResultID == "" {
			continue
		}
		if _, ok := seen[params.ResultID]; ok {
			continue
		}
		seen[params.ResultID] = struct{}{}
		out.Add(params.ResultID)
	}
	return out
}

func orderedExistingProbeResults(
	params *collectionlist.List[normalizedProbeResultParams],
	existing map[string]model.ProbeResult,
) []model.ProbeResult {
	values := params.Values()
	seen := make(map[string]struct{}, len(existing))
	out := make([]model.ProbeResult, 0, len(existing))
	for index := range values {
		params := values[index]
		if params.ResultID == "" {
			continue
		}
		result, ok := existing[params.ResultID]
		if !ok {
			continue
		}
		if _, ok := seen[params.ResultID]; ok {
			continue
		}
		seen[params.ResultID] = struct{}{}
		out = append(out, result)
	}
	return out
}

func pendingProbeResults(
	params *collectionlist.List[normalizedProbeResultParams],
	existing map[string]model.ProbeResult,
) *collectionlist.List[normalizedProbeResultParams] {
	values := params.Values()
	seen := make(map[string]struct{}, len(values))
	out := collectionlist.NewListWithCapacity[normalizedProbeResultParams](len(values))
	for index := range values {
		params := values[index]
		if params.ResultID != "" {
			if _, ok := existing[params.ResultID]; ok {
				continue
			}
			if _, ok := seen[params.ResultID]; ok {
				continue
			}
			seen[params.ResultID] = struct{}{}
		}
		out.Add(params)
	}
	return out
}
