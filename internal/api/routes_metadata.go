package api

import (
	"context"

	"github.com/arcgolabs/httpx"
	"github.com/lyonbrown4d/orivis/internal/buildinfo"
)

func (s *Server) registerMetadataRoutes() {
	httpx.MustGet(s.runtime, "/api/server/metadata", func(context.Context, *struct{}) (*metadataOutput, error) {
		out := &metadataOutput{}
		out.Body.Name = "orivis-server"
		out.Body.Env = s.cfg.App.Env
		out.Body.Version = buildinfo.Current()
		out.Body.Database.Driver = s.cfg.DB.Driver
		if s.store != nil && s.store.DB != nil && s.store.DB.Dialect() != nil {
			out.Body.Database.Dialect = s.store.DB.Dialect().Name()
		}
		return out, nil
	})
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
