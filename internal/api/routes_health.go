package api

import (
	"context"

	"github.com/arcgolabs/httpx"
)

func (e *healthEndpoint) Register(registrar httpx.Registrar) {
	scope := registrar.Scope()
	httpx.MustGroupGet(scope, "healthz", e.health)
	httpx.MustGroupGet(scope, "readyz", e.ready)
}

func (e *healthEndpoint) health(context.Context, *struct{}) (*statusOutput, error) {
	return newStatusOutput("ok"), nil
}

func (e *healthEndpoint) ready(context.Context, *struct{}) (*statusOutput, error) {
	return newStatusOutput("ready"), nil
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
