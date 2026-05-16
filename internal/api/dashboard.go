package api

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	collectionmapping "github.com/arcgolabs/collectionx/mapping"
	"github.com/danielgtaylor/huma/v2"
	"github.com/lyonbrown4d/orivis/internal/buildinfo"
	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/store"
)

type dashboardInput struct {
	Authorization string `header:"Authorization"`
}

type dashboardOutput struct {
	ContentType string `header:"Content-Type"`
	Body        []byte
}

type dashboardView struct {
	Name          string
	Env           string
	Version       buildinfo.Info
	Database      dashboardDatabase
	GeneratedAt   time.Time
	Agents        []store.DashboardAgent
	Monitors      []dashboardMonitorView
	Environments  []dashboardEnvironmentGroup
	RecentResults []dashboardResultView
	Summary       dashboardSummary
}

type dashboardDatabase struct {
	Driver  string
	Dialect string
}

type dashboardSummary struct {
	Agents   int
	Monitors int
	Up       int
	Down     int
	Unknown  int
}

type dashboardMonitorView struct {
	store.DashboardMonitor
	Latest *dashboardResultView
}

type dashboardEnvironmentGroup struct {
	Name     string
	Monitors []dashboardMonitorView
	Up       int
	Down     int
	Unknown  int
}

type dashboardResultView struct {
	store.DashboardResult
	MonitorName string
}

func (s *Server) verifyDashboardAuth(authorization string) error {
	cfg := s.cfg.Auth.Dashboard
	if !cfg.Enabled {
		return nil
	}

	username := strings.TrimSpace(cfg.Username)
	password := cfg.Password
	if username == "" || password == "" {
		return huma.Error500InternalServerError("dashboard auth is enabled but credentials are not configured")
	}

	const prefix = "Basic "
	if !strings.HasPrefix(authorization, prefix) {
		return dashboardUnauthorized()
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(strings.TrimPrefix(authorization, prefix)))
	if err != nil {
		return dashboardUnauthorized()
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return dashboardUnauthorized()
	}

	usernameOK := subtle.ConstantTimeCompare([]byte(parts[0]), []byte(username)) == 1
	passwordOK := subtle.ConstantTimeCompare([]byte(parts[1]), []byte(password)) == 1
	if !usernameOK || !passwordOK {
		return dashboardUnauthorized()
	}
	return nil
}

func dashboardUnauthorized() error {
	return huma.ErrorWithHeaders(
		huma.Error401Unauthorized("dashboard authentication required"),
		http.Header{"WWW-Authenticate": []string{`Basic realm="orivis"`}},
	)
}

func (s *Server) renderDashboard(ctx context.Context) ([]byte, error) {
	view := dashboardView{
		Name:        "orivis-server",
		Env:         s.cfg.App.Env,
		Version:     buildinfo.Current(),
		GeneratedAt: time.Now().UTC(),
		Database: dashboardDatabase{
			Driver: s.cfg.DB.Driver,
		},
	}
	if s.store != nil && s.store.DB != nil && s.store.DB.Dialect() != nil {
		view.Database.Dialect = s.store.DB.Dialect().Name()
	}

	if s.store != nil {
		snapshot, err := s.store.DashboardSnapshot(ctx, 50)
		if err != nil {
			return nil, huma.Error500InternalServerError("load dashboard snapshot", err)
		}
		view.GeneratedAt = snapshot.GeneratedAt
		view.Agents = snapshot.Agents
		view.Summary.Agents = len(snapshot.Agents)
		view.Summary.Monitors = len(snapshot.Monitors)
		view.Monitors = dashboardMonitors(snapshot)
		view.Environments = dashboardEnvironmentGroups(view.Monitors)
		view.RecentResults = dashboardResults(snapshot, 12)
		for _, monitor := range view.Monitors {
			switch {
			case monitor.Latest == nil:
				view.Summary.Unknown++
			case monitor.Latest.Status == model.StatusUp:
				view.Summary.Up++
			case monitor.Latest.Status == model.StatusDown:
				view.Summary.Down++
			default:
				view.Summary.Unknown++
			}
		}
	}

	var buf bytes.Buffer
	if err := dashboardTemplate.Execute(&buf, view); err != nil {
		return nil, fmt.Errorf("execute dashboard template: %w", err)
	}
	return buf.Bytes(), nil
}

func dashboardMonitors(snapshot store.DashboardSnapshot) []dashboardMonitorView {
	latestByMonitor := collectionmapping.NewMapWithCapacity[string, dashboardResultView](len(snapshot.Results))
	collectionlist.NewList(snapshot.Results...).Range(func(_ int, result store.DashboardResult) bool {
		if _, ok := latestByMonitor.Get(result.MonitorID); !ok {
			latestByMonitor.Set(result.MonitorID, dashboardResultView{DashboardResult: result})
		}
		return true
	})

	return collectionlist.MapList(
		collectionlist.NewList(snapshot.Monitors...),
		func(_ int, monitor store.DashboardMonitor) dashboardMonitorView {
			item := dashboardMonitorView{DashboardMonitor: monitor}
			if latest, ok := latestByMonitor.Get(monitor.ID); ok {
				latest.MonitorName = monitor.Name
				item.Latest = &latest
			}
			return item
		},
	).Values()
}

func dashboardEnvironmentGroups(monitors []dashboardMonitorView) []dashboardEnvironmentGroup {
	indexByName := collectionmapping.NewMapWithCapacity[string, int](len(monitors))
	groups := make([]dashboardEnvironmentGroup, 0)
	for _, monitor := range monitors {
		name := strings.TrimSpace(monitor.EnvironmentCode)
		if name == "" {
			name = "default"
		}
		index, ok := indexByName.Get(name)
		if !ok {
			index = len(groups)
			indexByName.Set(name, index)
			groups = append(groups, dashboardEnvironmentGroup{Name: name})
		}

		group := &groups[index]
		group.Monitors = append(group.Monitors, monitor)
		switch {
		case monitor.Latest == nil:
			group.Unknown++
		case monitor.Latest.Status == model.StatusUp:
			group.Up++
		case monitor.Latest.Status == model.StatusDown:
			group.Down++
		default:
			group.Unknown++
		}
	}
	return groups
}

func dashboardResults(snapshot store.DashboardSnapshot, limit int) []dashboardResultView {
	monitorNames := collectionmapping.AssociateList(
		collectionlist.NewList(snapshot.Monitors...),
		func(_ int, monitor store.DashboardMonitor) (string, string) {
			return monitor.ID, monitor.Name
		},
	)

	results := collectionlist.MapList(
		collectionlist.NewList(snapshot.Results...),
		func(_ int, result store.DashboardResult) dashboardResultView {
			return dashboardResultView{
				DashboardResult: result,
				MonitorName:     monitorNames.GetOrDefault(result.MonitorID, ""),
			}
		},
	).Values()
	if limit > 0 && len(results) > limit {
		return results[:limit]
	}
	return results
}

func dashboardStatusClass(status model.Status) string {
	switch status {
	case model.StatusUp:
		return "bg-emerald-100 text-emerald-700 ring-emerald-200"
	case model.StatusDown:
		return "bg-rose-100 text-rose-700 ring-rose-200"
	case model.StatusDegraded:
		return "bg-amber-100 text-amber-700 ring-amber-200"
	default:
		return "bg-slate-100 text-slate-600 ring-slate-200"
	}
}

func dashboardBoolLabel(value bool) string {
	if value {
		return "enabled"
	}
	return "disabled"
}

func dashboardDuration(value time.Duration) string {
	if value <= 0 {
		return "-"
	}
	if value%time.Second == 0 {
		return fmt.Sprintf("%ds", int(value.Seconds()))
	}
	return value.String()
}

func dashboardJoin(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ", ")
}

func dashboardSince(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	duration := time.Since(value)
	switch {
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		return fmt.Sprintf("%dm ago", int(duration.Minutes()))
	case duration < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(duration.Hours()))
	default:
		return value.UTC().Format("2006-01-02 15:04 UTC")
	}
}

var dashboardTemplate = template.Must(template.New("dashboard").Funcs(template.FuncMap{
	"statusClass": dashboardStatusClass,
	"boolLabel":   dashboardBoolLabel,
	"duration":    dashboardDuration,
	"join":        dashboardJoin,
	"since":       dashboardSince,
}).Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta http-equiv="refresh" content="15">
  <title>Orivis Uptime</title>
  <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="min-h-screen bg-[#f5f1e8] text-slate-950">
  <main class="mx-auto max-w-7xl px-5 py-8 sm:px-8">
    <section class="overflow-hidden rounded-[2rem] border border-slate-900/10 bg-[#10221f] text-white shadow-2xl shadow-slate-900/15">
      <div class="relative px-6 py-8 sm:px-10">
        <div class="absolute -right-20 -top-24 h-72 w-72 rounded-full bg-emerald-300/20 blur-3xl"></div>
        <div class="absolute bottom-0 right-24 h-40 w-40 rounded-full bg-amber-300/20 blur-2xl"></div>
        <div class="relative grid gap-8 lg:grid-cols-[1.4fr_1fr] lg:items-end">
          <div>
            <p class="text-sm uppercase tracking-[0.35em] text-emerald-200">distributed uptime</p>
            <h1 class="mt-4 text-4xl font-black tracking-tight sm:text-6xl">Orivis</h1>
            <p class="mt-4 max-w-2xl text-lg text-emerald-50/80">A zero-config server view for agents, discovered monitors, and recent probe results.</p>
          </div>
          <div class="grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-2">
            <div class="rounded-2xl bg-white/10 p-4 ring-1 ring-white/10">
              <p class="text-sm text-emerald-100">Agents</p>
              <p class="mt-2 text-3xl font-black">{{.Summary.Agents}}</p>
            </div>
            <div class="rounded-2xl bg-white/10 p-4 ring-1 ring-white/10">
              <p class="text-sm text-emerald-100">Monitors</p>
              <p class="mt-2 text-3xl font-black">{{.Summary.Monitors}}</p>
            </div>
            <div class="rounded-2xl bg-white/10 p-4 ring-1 ring-white/10">
              <p class="text-sm text-emerald-100">Up</p>
              <p class="mt-2 text-3xl font-black text-emerald-200">{{.Summary.Up}}</p>
            </div>
            <div class="rounded-2xl bg-white/10 p-4 ring-1 ring-white/10">
              <p class="text-sm text-emerald-100">Down</p>
              <p class="mt-2 text-3xl font-black text-rose-200">{{.Summary.Down}}</p>
            </div>
          </div>
        </div>
      </div>
    </section>

    <section class="mt-6 grid gap-4 lg:grid-cols-3">
      <div class="rounded-3xl border border-slate-900/10 bg-white/80 p-5 shadow-sm">
        <p class="text-xs uppercase tracking-[0.25em] text-slate-500">server</p>
        <p class="mt-2 font-semibold">{{.Name}}</p>
        <p class="text-sm text-slate-500">env: {{.Env}}</p>
      </div>
      <div class="rounded-3xl border border-slate-900/10 bg-white/80 p-5 shadow-sm">
        <p class="text-xs uppercase tracking-[0.25em] text-slate-500">storage</p>
        <p class="mt-2 font-semibold">{{.Database.Driver}}</p>
        <p class="text-sm text-slate-500">{{if .Database.Dialect}}{{.Database.Dialect}}{{else}}memory{{end}}</p>
      </div>
      <div class="rounded-3xl border border-slate-900/10 bg-white/80 p-5 shadow-sm">
        <p class="text-xs uppercase tracking-[0.25em] text-slate-500">updated</p>
        <p class="mt-2 font-semibold">{{since .GeneratedAt}}</p>
        <p class="text-sm text-slate-500">{{.GeneratedAt.Format "2006-01-02 15:04:05 UTC"}} / auto refresh 15s</p>
      </div>
    </section>

    <section class="mt-8 grid gap-6 xl:grid-cols-[1.45fr_0.9fr]">
      <div class="rounded-[1.75rem] border border-slate-900/10 bg-white p-5 shadow-sm">
        <div class="flex items-center justify-between gap-4">
          <h2 class="text-xl font-black">Monitors</h2>
          <span class="rounded-full bg-slate-100 px-3 py-1 text-sm text-slate-600">{{.Summary.Monitors}} total</span>
        </div>
        <div class="mt-5 overflow-x-auto">
          <table class="w-full min-w-[760px] text-left text-sm">
            <thead class="text-xs uppercase tracking-wider text-slate-500">
              <tr class="border-b border-slate-200">
                <th class="py-3 pr-4">Name</th>
                <th class="py-3 pr-4">Target</th>
                <th class="py-3 pr-4">Env</th>
                <th class="py-3 pr-4">Interval</th>
                <th class="py-3 pr-4">Status</th>
                <th class="py-3 pr-4">Latency</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100">
              {{range .Environments}}
              <tr class="bg-slate-50">
                <td class="py-3 pr-4 font-black text-slate-700" colspan="6">
                  <span class="rounded-full bg-white px-3 py-1 ring-1 ring-slate-200">{{.Name}}</span>
                  <span class="ml-2 text-xs font-semibold text-emerald-700">{{.Up}} up</span>
                  <span class="ml-2 text-xs font-semibold text-rose-700">{{.Down}} down</span>
                  <span class="ml-2 text-xs font-semibold text-slate-500">{{.Unknown}} unknown</span>
                </td>
              </tr>
              {{range .Monitors}}
              <tr>
                <td class="py-4 pr-4">
                  <div class="font-semibold">{{.Name}}</div>
                  <div class="text-xs text-slate-500">{{.Type}} / {{boolLabel .Enabled}}</div>
                </td>
                <td class="max-w-[320px] truncate py-4 pr-4 text-slate-600">{{.Target}}</td>
                <td class="py-4 pr-4">{{.EnvironmentCode}}</td>
                <td class="py-4 pr-4">{{duration .Interval}}</td>
                <td class="py-4 pr-4">
                  {{if .Latest}}
                  <span class="rounded-full px-2.5 py-1 text-xs font-bold ring-1 {{statusClass .Latest.Status}}">{{.Latest.Status}}</span>
                  {{else}}
                  <span class="rounded-full bg-slate-100 px-2.5 py-1 text-xs font-bold text-slate-600 ring-1 ring-slate-200">unknown</span>
                  {{end}}
                </td>
                <td class="py-4 pr-4">{{if .Latest}}{{duration .Latest.Latency}}{{else}}-{{end}}</td>
              </tr>
              {{end}}
              {{else}}
              <tr>
                <td class="py-10 text-center text-slate-500" colspan="6">No monitors yet. Start an agent with static config or Docker labels.</td>
              </tr>
              {{end}}
            </tbody>
          </table>
        </div>
      </div>

      <div class="space-y-6">
        <div class="rounded-[1.75rem] border border-slate-900/10 bg-white p-5 shadow-sm">
          <h2 class="text-xl font-black">Agents</h2>
          <div class="mt-4 space-y-3">
            {{range .Agents}}
            <div class="rounded-2xl border border-slate-100 bg-slate-50 p-4">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <p class="font-bold">{{.Name}}</p>
                  <p class="text-sm text-slate-500">{{.RuntimeType}} / {{.RegionCode}}</p>
                </div>
                <span class="rounded-full bg-emerald-100 px-2.5 py-1 text-xs font-bold text-emerald-700 ring-1 ring-emerald-200">{{.Status}}</span>
              </div>
              <p class="mt-3 text-sm text-slate-500">env: {{join .EnvironmentCodes}}</p>
              <p class="text-sm text-slate-500">last seen: {{since .LastSeenAt}}</p>
            </div>
            {{else}}
            <p class="rounded-2xl bg-slate-50 p-4 text-sm text-slate-500">No agents registered yet.</p>
            {{end}}
          </div>
        </div>

        <div class="rounded-[1.75rem] border border-slate-900/10 bg-white p-5 shadow-sm">
          <h2 class="text-xl font-black">Recent results</h2>
          <div class="mt-4 space-y-3">
            {{range .RecentResults}}
            <div class="rounded-2xl border border-slate-100 p-4">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <p class="font-bold">{{if .MonitorName}}{{.MonitorName}}{{else}}{{.MonitorID}}{{end}}</p>
                  <p class="text-sm text-slate-500">{{.AgentName}} / {{.EnvironmentCode}}</p>
                </div>
                <span class="rounded-full px-2.5 py-1 text-xs font-bold ring-1 {{statusClass .Status}}">{{.Status}}</span>
              </div>
              <p class="mt-3 text-sm text-slate-500">{{duration .Latency}} / {{since .CheckedAt}}</p>
              {{if .ErrorMessage}}<p class="mt-2 rounded-xl bg-rose-50 p-3 text-sm text-rose-700">{{.ErrorMessage}}</p>{{end}}
            </div>
            {{else}}
            <p class="rounded-2xl bg-slate-50 p-4 text-sm text-slate-500">No probe results yet.</p>
            {{end}}
          </div>
        </div>
      </div>
    </section>
  </main>
</body>
</html>`))
