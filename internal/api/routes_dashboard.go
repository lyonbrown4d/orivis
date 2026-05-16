package api

import (
	"context"

	"github.com/arcgolabs/httpx"
)

func (e *dashboardEndpoint) Register(registrar httpx.Registrar) {
	httpx.MustGroupGet(registrar.Scope(), "", e.index)
}

func (e *dashboardEndpoint) index(ctx context.Context, input *dashboardInput) (*dashboardOutput, error) {
	if err := e.verifyDashboardAuth(input.Authorization); err != nil {
		return nil, err
	}

	html, err := e.renderDashboardPage(ctx, input.Lang)
	if err != nil {
		return nil, err
	}
	return &dashboardOutput{
		ContentType: "text/html; charset=utf-8",
		Body:        html,
	}, nil
}
