package api

import (
	"context"

	"github.com/arcgolabs/httpx"
)

func (s *Server) registerHealthRoutes() {
	httpx.MustGet(s.runtime, "/healthz", func(context.Context, *struct{}) (*statusOutput, error) {
		return newStatusOutput("ok"), nil
	})

	httpx.MustGet(s.runtime, "/readyz", func(context.Context, *struct{}) (*statusOutput, error) {
		return newStatusOutput("ready"), nil
	})
}

type statusOutput struct {
	Body struct {
		Status string `json:"status"`
	} `json:"body"`
}

func newStatusOutput(status string) *statusOutput {
	out := &statusOutput{}
	out.Body.Status = status
	return out
}
