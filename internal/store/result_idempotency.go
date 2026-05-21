package store

import (
	"context"

	collectionlist "github.com/arcgolabs/collectionx/list"
	collectionset "github.com/arcgolabs/collectionx/set"
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
		return nil, wrapError(err, "find existing probe results")
	}

	values := rows.Values()
	out := make(map[string]model.ProbeResult, rows.Len())
	for index := range values {
		row := values[index]
		result, err := row.model()
		if err != nil {
			return nil, wrapError(err, "map existing probe result")
		}
		out[row.ResultID] = result
	}
	return out, nil
}

func uniqueResultIDs(params *collectionlist.List[normalizedProbeResultParams]) *collectionlist.List[string] {
	seen := collectionset.NewSetWithCapacity[string](params.Len())
	return collectionlist.FilterMapList(params, func(_ int, params normalizedProbeResultParams) (string, bool) {
		if params.ResultID == "" {
			return "", false
		}
		if seen.Contains(params.ResultID) {
			return "", false
		}
		seen.Add(params.ResultID)
		return params.ResultID, true
	})
}

func orderedExistingProbeResults(
	params *collectionlist.List[normalizedProbeResultParams],
	existing map[string]model.ProbeResult,
) []model.ProbeResult {
	seen := collectionset.NewSetWithCapacity[string](len(existing))
	return collectionlist.FilterMapList(params, func(_ int, params normalizedProbeResultParams) (model.ProbeResult, bool) {
		if params.ResultID == "" {
			return model.ProbeResult{}, false
		}
		result, ok := existing[params.ResultID]
		if !ok {
			return model.ProbeResult{}, false
		}
		if seen.Contains(params.ResultID) {
			return model.ProbeResult{}, false
		}
		seen.Add(params.ResultID)
		return result, true
	}).Values()
}

func pendingProbeResults(
	params *collectionlist.List[normalizedProbeResultParams],
	existing map[string]model.ProbeResult,
) *collectionlist.List[normalizedProbeResultParams] {
	seen := collectionset.NewSetWithCapacity[string](params.Len())
	return collectionlist.FilterMapList(params, func(_ int, params normalizedProbeResultParams) (normalizedProbeResultParams, bool) {
		if params.ResultID != "" {
			if _, ok := existing[params.ResultID]; ok {
				return normalizedProbeResultParams{}, false
			}
			if seen.Contains(params.ResultID) {
				return normalizedProbeResultParams{}, false
			}
			seen.Add(params.ResultID)
		}
		return params, true
	})
}
