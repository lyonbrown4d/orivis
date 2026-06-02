// Package ui embeds the server-rendered Orivis dashboard templates and static assets.
package ui

import (
	"embed"
	"fmt"
	"html/template"
)

const (
	TemplateDashboard     = "dashboard.html"
	TemplateStatus        = "status.html"
	TemplateLogin         = "login.html"
	TemplateMonitorDetail = "monitor_detail.html"
)

// Templates contains the server-rendered HTML templates for the built-in UI.
//
// Intended template entry points:
//   - dashboard.html
//   - status.html
//   - login.html
//   - monitor_detail.html
//
// Route registration is intentionally left to the server assembly layer.
//
//go:embed templates/*.html templates/layouts/*.html templates/partials/*.html
var Templates embed.FS

// Static contains small UI assets that can be served by the server when the UI
// routes are wired later.
//
//go:embed static/* static/css/*
var Static embed.FS

// ParseTemplate parses one page entry with the shared layout and partials.
func ParseTemplate(entry string) (*template.Template, error) {
	switch entry {
	case TemplateDashboard, TemplateStatus, TemplateLogin, TemplateMonitorDetail:
	default:
		return nil, fmt.Errorf("unknown UI template entry %q", entry)
	}

	parsed, err := template.ParseFS(
		Templates,
		"templates/layouts/base.html",
		"templates/partials/nav.html",
		"templates/"+entry,
	)
	if err != nil {
		return nil, fmt.Errorf("parse UI template %q: %w", entry, err)
	}
	tmpl := parsed.Lookup(entry)
	if tmpl == nil {
		return nil, fmt.Errorf("UI template entry %q was not parsed", entry)
	}
	return tmpl, nil
}
