package api

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/danielgtaylor/huma/v2"
)

type dashboardAssetInput struct {
	Name string `path:"name"`
}

type dashboardAssetOutput struct {
	ContentType  string `header:"Content-Type"`
	CacheControl string `header:"Cache-Control"`
	Body         []byte
}

func (e *dashboardEndpoint) asset(_ context.Context, input *dashboardAssetInput) (*dashboardAssetOutput, error) {
	contentType, ok := dashboardAssetContentType(input.Name)
	if !ok {
		return nil, huma.Error404NotFound("dashboard asset not found")
	}
	content, err := fs.ReadFile(dashboardTemplateFS, "assets/"+input.Name)
	if err != nil {
		return nil, fmt.Errorf("read dashboard asset %s: %w", input.Name, err)
	}
	return &dashboardAssetOutput{
		ContentType:  contentType,
		CacheControl: "public, max-age=300",
		Body:         content,
	}, nil
}

func dashboardAssetContentType(name string) (string, bool) {
	switch name {
	case "dashboard.css":
		return "text/css; charset=utf-8", true
	case "dashboard-head.js", "dashboard.js":
		return "application/javascript; charset=utf-8", true
	default:
		return "", false
	}
}
