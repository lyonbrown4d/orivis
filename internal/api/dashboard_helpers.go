package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/lyonbrown4d/orivis/internal/model"
)

func dashboardStatusClass(status model.Status) string {
	switch status {
	case model.StatusUp:
		return "bg-emerald-100 text-emerald-700 ring-emerald-200"
	case model.StatusDown:
		return "bg-rose-100 text-rose-700 ring-rose-200"
	case model.StatusDegraded:
		return "bg-amber-100 text-amber-700 ring-amber-200"
	case model.StatusUnknown:
		return "bg-slate-100 text-slate-600 ring-slate-200"
	default:
		return "bg-slate-100 text-slate-600 ring-slate-200"
	}
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

func dashboardSince(t func(string) string, value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	duration := time.Since(value)
	switch {
	case duration < time.Minute:
		return dashboardText(t, "time.just_now", "just now")
	case duration < time.Hour:
		return dashboardFormat(t, "time.minutes_ago", "%dm ago", int(duration.Minutes()))
	case duration < 24*time.Hour:
		return dashboardFormat(t, "time.hours_ago", "%dh ago", int(duration.Hours()))
	default:
		return value.UTC().Format("2006-01-02 15:04 UTC")
	}
}

func dashboardText(t func(string) string, key, fallback string) string {
	if t == nil {
		return fallback
	}
	text := t(key)
	if strings.TrimSpace(text) == "" || text == key {
		return fallback
	}
	return text
}

func dashboardFormat(t func(string) string, key, fallback string, args ...any) string {
	return fmt.Sprintf(dashboardText(t, key, fallback), args...)
}
