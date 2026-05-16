package store

import (
	"github.com/arcgolabs/dbx"
	columnx "github.com/arcgolabs/dbx/column"
	repository "github.com/arcgolabs/dbx/repository"
	schemax "github.com/arcgolabs/dbx/schema"
)

var (
	regionsSchema           = schemax.MustSchema("regions", regionSchema{})
	environmentsSchema      = schemax.MustSchema("environments", environmentSchema{})
	agentsSchema            = schemax.MustSchema("agents", agentSchema{})
	agentEnvironmentsSchema = schemax.MustSchema("agent_environments", agentEnvironmentSchema{})
)

type regionRow struct {
	ID          string `dbx:"id"`
	Name        string `dbx:"name"`
	Code        string `dbx:"code"`
	Description string `dbx:"description"`
	Enabled     int    `dbx:"enabled"`
	Source      string `dbx:"source"`
	CreatedAt   string `dbx:"created_at"`
	UpdatedAt   string `dbx:"updated_at"`
}

type regionSchema struct {
	schemax.Schema[regionRow]
	ID          columnx.Column[regionRow, string] `dbx:"id,pk"`
	Name        columnx.Column[regionRow, string] `dbx:"name"`
	Code        columnx.Column[regionRow, string] `dbx:"code,unique"`
	Description columnx.Column[regionRow, string] `dbx:"description"`
	Enabled     columnx.Column[regionRow, int]    `dbx:"enabled"`
	Source      columnx.Column[regionRow, string] `dbx:"source"`
	CreatedAt   columnx.Column[regionRow, string] `dbx:"created_at"`
	UpdatedAt   columnx.Column[regionRow, string] `dbx:"updated_at"`
}

type environmentRow struct {
	ID          string `dbx:"id"`
	Name        string `dbx:"name"`
	Code        string `dbx:"code"`
	Description string `dbx:"description"`
	Enabled     int    `dbx:"enabled"`
	Source      string `dbx:"source"`
	CreatedAt   string `dbx:"created_at"`
	UpdatedAt   string `dbx:"updated_at"`
}

type environmentSchema struct {
	schemax.Schema[environmentRow]
	ID          columnx.Column[environmentRow, string] `dbx:"id,pk"`
	Name        columnx.Column[environmentRow, string] `dbx:"name"`
	Code        columnx.Column[environmentRow, string] `dbx:"code,unique"`
	Description columnx.Column[environmentRow, string] `dbx:"description"`
	Enabled     columnx.Column[environmentRow, int]    `dbx:"enabled"`
	Source      columnx.Column[environmentRow, string] `dbx:"source"`
	CreatedAt   columnx.Column[environmentRow, string] `dbx:"created_at"`
	UpdatedAt   columnx.Column[environmentRow, string] `dbx:"updated_at"`
}

type agentSchema struct {
	schemax.Schema[agentRecord]
	ID          columnx.Column[agentRecord, string] `dbx:"id,pk"`
	Name        columnx.Column[agentRecord, string] `dbx:"name,unique"`
	TokenHash   columnx.Column[agentRecord, string] `dbx:"token_hash"`
	RegionID    columnx.Column[agentRecord, string] `dbx:"region_id"`
	RuntimeType columnx.Column[agentRecord, string] `dbx:"runtime_type"`
	Version     columnx.Column[agentRecord, string] `dbx:"version"`
	LastSeenAt  columnx.Column[agentRecord, string] `dbx:"last_seen_at"`
	Status      columnx.Column[agentRecord, string] `dbx:"status"`
	Source      columnx.Column[agentRecord, string] `dbx:"source"`
	CreatedAt   columnx.Column[agentRecord, string] `dbx:"created_at"`
	UpdatedAt   columnx.Column[agentRecord, string] `dbx:"updated_at"`
}

type agentEnvironmentRow struct {
	AgentID       string `dbx:"agent_id"`
	EnvironmentID string `dbx:"environment_id"`
}

type agentEnvironmentSchema struct {
	schemax.Schema[agentEnvironmentRow]
	AgentID       columnx.Column[agentEnvironmentRow, string] `dbx:"agent_id"`
	EnvironmentID columnx.Column[agentEnvironmentRow, string] `dbx:"environment_id"`
	PK            schemax.CompositeKey[agentEnvironmentRow]   `key:"columns=agent_id|environment_id"`
}

func newRegionRepository(database *dbx.DB) *repository.Base[regionRow, regionSchema] {
	return repository.New[regionRow](database, regionsSchema)
}

func newEnvironmentRepository(database *dbx.DB) *repository.Base[environmentRow, environmentSchema] {
	return repository.New[environmentRow](database, environmentsSchema)
}

func newAgentRepository(database *dbx.DB) *repository.Base[agentRecord, agentSchema] {
	return repository.New[agentRecord](database, agentsSchema)
}

func newAgentEnvironmentRepository(database *dbx.DB) *repository.Base[agentEnvironmentRow, agentEnvironmentSchema] {
	return repository.New[agentEnvironmentRow](database, agentEnvironmentsSchema)
}

type Repositories struct {
	regions           *repository.Base[regionRow, regionSchema]
	environments      *repository.Base[environmentRow, environmentSchema]
	agents            *repository.Base[agentRecord, agentSchema]
	agentEnvironments *repository.Base[agentEnvironmentRow, agentEnvironmentSchema]
	monitors          *repository.Base[monitorRecord, monitorSchema]
	monitorAgents     *repository.Base[monitorAgentRow, monitorAgentSchema]
	probeResults      *repository.Base[probeResultRow, probeResultSchema]
}

func NewRepositories(database *dbx.DB) *Repositories {
	return &Repositories{
		regions:           newRegionRepository(database),
		environments:      newEnvironmentRepository(database),
		agents:            newAgentRepository(database),
		agentEnvironments: newAgentEnvironmentRepository(database),
		monitors:          newMonitorRepository(database),
		monitorAgents:     newMonitorAgentRepository(database),
		probeResults:      newProbeResultRepository(database),
	}
}
