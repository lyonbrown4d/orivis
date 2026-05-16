package api

import (
	"context"

	"github.com/arcgolabs/httpx"
	"github.com/lyonbrown4d/orivis/internal/buildinfo"
)

func (e *metadataEndpoint) Register(registrar httpx.Registrar) {
	httpx.MustGroupGet(registrar.Scope(), "api/server/metadata", e.metadata)
}

func (e *metadataEndpoint) metadata(context.Context, *struct{}) (*metadataOutput, error) {
	out := &metadataOutput{}
	out.Body.Name = "orivis-server"
	out.Body.Env = e.cfg.App.Env
	out.Body.Version = buildinfo.Current()
	out.Body.Database.Driver = e.cfg.DB.Driver
	if e.store != nil && e.store.DB != nil && e.store.DB.Dialect() != nil {
		out.Body.Database.Dialect = e.store.DB.Dialect().Name()
	}
	return out, nil
}

type metadataOutput struct {
	Body struct {
		Name     string         `json:"name"`
		Env      string         `json:"env"`
		Version  buildinfo.Info `json:"version"`
		Database struct {
			Driver  string `json:"driver"`
			Dialect string `json:"dialect,omitempty"`
		} `json:"database"`
	} `json:"body"`
}
