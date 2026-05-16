package store

import "github.com/hashicorp/go-memdb"

func memorySchema() *memdb.DBSchema {
	return &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			memoryTableAgents: {
				Name: memoryTableAgents,
				Indexes: map[string]*memdb.IndexSchema{
					"id":   uniqueStringIndex("id", "ID"),
					"name": uniqueStringIndex("name", "Name"),
				},
			},
			memoryTableRegions: {
				Name: memoryTableRegions,
				Indexes: map[string]*memdb.IndexSchema{
					"id":   uniqueStringIndex("id", "ID"),
					"code": uniqueStringIndex("code", "Code"),
				},
			},
			memoryTableEnvironments: {
				Name: memoryTableEnvironments,
				Indexes: map[string]*memdb.IndexSchema{
					"id":   uniqueStringIndex("id", "ID"),
					"code": uniqueStringIndex("code", "Code"),
				},
			},
			memoryTableMonitors: {
				Name: memoryTableMonitors,
				Indexes: map[string]*memdb.IndexSchema{
					"id":         uniqueStringIndex("id", "ID"),
					"source_key": optionalStringIndex("source_key", "SourceKey"),
				},
			},
			memoryTableAgentEnvironments: {
				Name: memoryTableAgentEnvironments,
				Indexes: map[string]*memdb.IndexSchema{
					"id":    uniqueStringIndex("id", "ID"),
					"agent": stringIndex("agent", "AgentID"),
				},
			},
			memoryTableMonitorAgents: {
				Name: memoryTableMonitorAgents,
				Indexes: map[string]*memdb.IndexSchema{
					"id":      uniqueStringIndex("id", "ID"),
					"agent":   stringIndex("agent", "AgentID"),
					"monitor": stringIndex("monitor", "MonitorID"),
				},
			},
			memoryTableProbeResults: {
				Name: memoryTableProbeResults,
				Indexes: map[string]*memdb.IndexSchema{
					"id": uniqueStringIndex("id", "ID"),
				},
			},
		},
	}
}

func uniqueStringIndex(name, field string) *memdb.IndexSchema {
	return &memdb.IndexSchema{
		Name:    name,
		Unique:  true,
		Indexer: &memdb.StringFieldIndex{Field: field},
	}
}

func stringIndex(name, field string) *memdb.IndexSchema {
	return &memdb.IndexSchema{
		Name:    name,
		Indexer: &memdb.StringFieldIndex{Field: field},
	}
}

func optionalStringIndex(name, field string) *memdb.IndexSchema {
	return &memdb.IndexSchema{
		Name:         name,
		AllowMissing: true,
		Indexer:      &memdb.StringFieldIndex{Field: field},
	}
}
