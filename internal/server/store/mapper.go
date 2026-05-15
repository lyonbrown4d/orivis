package store

import (
	"time"

	mapperx "github.com/arcgolabs/dbx/mapper"
)

type MonitorRecord struct {
	ID            string
	Name          string
	Type          string
	Target        string
	EnvironmentID string
	Enabled       bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func NewMonitorMapper() (mapperx.StructMapper[MonitorRecord], error) {
	return mapperx.NewStructMapper[MonitorRecord]()
}
