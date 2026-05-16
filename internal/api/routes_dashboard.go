package api

import (
	"context"

	"github.com/arcgolabs/httpx"
)

func (s *Server) registerDashboardRoutes() {
	httpx.MustGet(s.runtime, "/", func(ctx context.Context, input *dashboardInput) (*dashboardOutput, error) {
		if err := s.verifyDashboardAuth(input.Authorization); err != nil {
			return nil, err
		}

		html, err := s.renderDashboardPage(ctx, input.Lang)
		if err != nil {
			return nil, err
		}
		return &dashboardOutput{
			ContentType: "text/html; charset=utf-8",
			Body:        html,
		}, nil
	})
}
