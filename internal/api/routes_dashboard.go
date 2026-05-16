package api

import (
	"context"
	"strings"

	"github.com/arcgolabs/httpx"
	"github.com/danielgtaylor/huma/v2"
)

func (e *dashboardEndpoint) Register(registrar httpx.Registrar) {
	scope := registrar.Scope()
	httpx.MustGroupGet(scope, "", e.index)
	httpx.MustGroupGet(scope, "{group}", e.group)
}

func (e *dashboardEndpoint) index(ctx context.Context, input *dashboardInput) (*dashboardOutput, error) {
	if err := e.verifyDashboardAuth(input.Authorization); err != nil {
		return nil, err
	}

	html, err := e.renderDashboardPage(ctx, input.Lang, "")
	if err != nil {
		return nil, err
	}
	return &dashboardOutput{
		ContentType: "text/html; charset=utf-8",
		Body:        html,
	}, nil
}

func (e *dashboardEndpoint) group(ctx context.Context, input *dashboardGroupInput) (*dashboardOutput, error) {
	if dashboardReservedGroup(input.Group) {
		return nil, huma.Error404NotFound("dashboard group not found")
	}
	if err := e.verifyDashboardAuth(input.Authorization); err != nil {
		return nil, err
	}

	html, err := e.renderDashboardPage(ctx, input.Lang, input.Group)
	if err != nil {
		return nil, err
	}
	return &dashboardOutput{
		ContentType: "text/html; charset=utf-8",
		Body:        html,
	}, nil
}

func dashboardReservedGroup(group string) bool {
	switch strings.ToLower(strings.TrimSpace(group)) {
	case "api", "healthz", "readyz", "docs", "openapi.json", "openapi.yaml":
		return true
	default:
		return false
	}
}
