package store

import (
	"time"

	"github.com/arcgolabs/dbx"
	columnx "github.com/arcgolabs/dbx/column"
	repository "github.com/arcgolabs/dbx/repository"
	schemax "github.com/arcgolabs/dbx/schema"
	"github.com/lyonbrown4d/orivis/internal/model"
)

type probeResultRow struct {
	ID            string `dbx:"id"`
	MonitorID     string `dbx:"monitor_id"`
	AgentID       string `dbx:"agent_id"`
	RegionID      string `dbx:"region_id"`
	EnvironmentID string `dbx:"environment_id"`
	Status        string `dbx:"status"`
	LatencyMS     int64  `dbx:"latency_ms"`
	ErrorMessage  string `dbx:"error_message"`
	CheckedAt     string `dbx:"checked_at"`
	RawDetail     []byte `dbx:"raw_detail"`
	CreatedAt     string `dbx:"created_at"`
}

type probeResultSchema struct {
	schemax.Schema[probeResultRow]
	ID            columnx.Column[probeResultRow, string] `dbx:"id,pk"`
	MonitorID     columnx.Column[probeResultRow, string] `dbx:"monitor_id"`
	AgentID       columnx.Column[probeResultRow, string] `dbx:"agent_id"`
	RegionID      columnx.Column[probeResultRow, string] `dbx:"region_id"`
	EnvironmentID columnx.Column[probeResultRow, string] `dbx:"environment_id"`
	Status        columnx.Column[probeResultRow, string] `dbx:"status"`
	LatencyMS     columnx.Column[probeResultRow, int64]  `dbx:"latency_ms"`
	ErrorMessage  columnx.Column[probeResultRow, string] `dbx:"error_message"`
	CheckedAt     columnx.Column[probeResultRow, string] `dbx:"checked_at"`
	RawDetail     columnx.Column[probeResultRow, []byte] `dbx:"raw_detail"`
	CreatedAt     columnx.Column[probeResultRow, string] `dbx:"created_at"`
}

func newProbeResultRepository(database *dbx.DB) *repository.Base[probeResultRow, probeResultSchema] {
	return repository.New[probeResultRow](database, probeResultSchemaResource())
}

func probeResultSchemaResource() probeResultSchema {
	return schemax.MustSchema("probe_results", probeResultSchema{})
}

func newProbeResultRow(
	id string,
	normalized normalizedProbeResultParams,
	monitor model.Monitor,
	now time.Time,
) *probeResultRow {
	return &probeResultRow{
		ID:            id,
		MonitorID:     monitor.ID,
		AgentID:       normalized.Agent.ID,
		RegionID:      normalized.Agent.RegionID,
		EnvironmentID: monitor.EnvironmentID,
		Status:        string(normalized.Status),
		LatencyMS:     normalized.Latency.Milliseconds(),
		ErrorMessage:  normalized.ErrorMessage,
		CheckedAt:     formatTime(normalized.CheckedAt),
		RawDetail:     append([]byte(nil), normalized.RawDetail...),
		CreatedAt:     formatTime(now),
	}
}
