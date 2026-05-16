package store

import (
	"github.com/arcgolabs/dbx"
	columnx "github.com/arcgolabs/dbx/column"
	repository "github.com/arcgolabs/dbx/repository"
	schemax "github.com/arcgolabs/dbx/schema"
)

var (
	monitorsSchema      = schemax.MustSchema("monitors", monitorSchema{})
	monitorAgentsSchema = schemax.MustSchema("monitor_agents", monitorAgentSchema{})
)

type monitorSchema struct {
	schemax.Schema[monitorRecord]
	ID                columnx.Column[monitorRecord, string] `dbx:"id,pk"`
	SourceKey         columnx.Column[monitorRecord, string] `dbx:"source_key"`
	Name              columnx.Column[monitorRecord, string] `dbx:"name"`
	Type              columnx.Column[monitorRecord, string] `dbx:"type"`
	Target            columnx.Column[monitorRecord, string] `dbx:"target"`
	GroupName         columnx.Column[monitorRecord, string] `dbx:"group_name"`
	EnvironmentID     columnx.Column[monitorRecord, string] `dbx:"environment_id"`
	Enabled           columnx.Column[monitorRecord, int]    `dbx:"enabled"`
	IntervalSeconds   columnx.Column[monitorRecord, int]    `dbx:"interval_seconds"`
	TimeoutSeconds    columnx.Column[monitorRecord, int]    `dbx:"timeout_seconds"`
	RetryCount        columnx.Column[monitorRecord, int]    `dbx:"retry_count"`
	AggregationPolicy columnx.Column[monitorRecord, string] `dbx:"aggregation_policy"`
	Source            columnx.Column[monitorRecord, string] `dbx:"source"`
	CreatedAt         columnx.Column[monitorRecord, string] `dbx:"created_at"`
	UpdatedAt         columnx.Column[monitorRecord, string] `dbx:"updated_at"`
}

type monitorAgentRow struct {
	MonitorID string `dbx:"monitor_id"`
	AgentID   string `dbx:"agent_id"`
}

type monitorAgentSchema struct {
	schemax.Schema[monitorAgentRow]
	MonitorID columnx.Column[monitorAgentRow, string] `dbx:"monitor_id"`
	AgentID   columnx.Column[monitorAgentRow, string] `dbx:"agent_id"`
	PK        schemax.CompositeKey[monitorAgentRow]   `key:"columns=monitor_id|agent_id"`
}

func newMonitorRepository(database *dbx.DB) *repository.Base[monitorRecord, monitorSchema] {
	return repository.New[monitorRecord](database, monitorsSchema)
}

func newMonitorAgentRepository(database *dbx.DB) *repository.Base[monitorAgentRow, monitorAgentSchema] {
	return repository.New[monitorAgentRow](database, monitorAgentsSchema)
}
