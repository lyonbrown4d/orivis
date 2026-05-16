package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/arcgolabs/httpx"
	"github.com/danielgtaylor/huma/v2"
)

func (e *dashboardEndpoint) Register(registrar httpx.Registrar) {
	scope := registrar.Scope()
	httpx.MustGroupGet(scope, "", e.index)
	httpx.MustGroupGet(scope, "assets/{name}", e.asset)
	httpx.MustGroupPost(scope, "login", e.login)
	httpx.MustGroupPost(scope, "logout", e.logout)
	httpx.MustGroupGet(scope, "{group}", e.group)
}

func (e *dashboardEndpoint) index(ctx context.Context, input *dashboardInput) (*dashboardOutput, error) {
	if !e.authenticateDashboardSession(input.SessionCookie) {
		return e.renderLoginOutput(input.Lang, input.AcceptLanguage, "")
	}

	html, err := e.renderDashboardPage(ctx, input.Lang, input.AcceptLanguage, "")
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
	if !e.authenticateDashboardSession(input.SessionCookie) {
		return e.renderLoginOutput(input.Lang, input.AcceptLanguage, input.Group)
	}

	html, err := e.renderDashboardPage(ctx, input.Lang, input.AcceptLanguage, input.Group)
	if err != nil {
		return nil, err
	}
	return &dashboardOutput{
		ContentType: "text/html; charset=utf-8",
		Body:        html,
	}, nil
}

func (e *dashboardEndpoint) login(ctx context.Context, input *dashboardLoginInput) (*dashboardLoginOutput, error) {
	if !e.cfg.Auth.Dashboard.Enabled {
		out := &dashboardLoginOutput{}
		out.Body.OK = true
		return out, nil
	}

	token, err := e.loginDashboard(ctx, input.Body.Username, input.Body.Password)
	if err != nil {
		if errors.Is(err, errDashboardLoginFailed) {
			return nil, huma.Error401Unauthorized("invalid username or password")
		}
		return nil, err
	}
	out := &dashboardLoginOutput{
		SetCookie: e.dashboardSessionSetCookie(token, false),
	}
	out.Body.OK = true
	return out, nil
}

func (e *dashboardEndpoint) logout(_ context.Context, input *dashboardLogoutInput) (*dashboardLogoutOutput, error) {
	e.sessions.Delete(input.SessionCookie)
	return &dashboardLogoutOutput{
		SetCookie: e.dashboardSessionSetCookie("", true),
		Location:  "/",
		Status:    http.StatusFound,
	}, nil
}

func (e *dashboardEndpoint) renderLoginOutput(lang, acceptLanguage, group string) (*dashboardOutput, error) {
	html, err := e.renderLoginPage(lang, acceptLanguage, group)
	if err != nil {
		return nil, err
	}
	return &dashboardOutput{
		ContentType: "text/html; charset=utf-8",
		Body:        html,
	}, nil
}

func (e *dashboardEndpoint) dashboardSessionSetCookie(token string, expired bool) string {
	maxAge := int(dashboardSessionTTL.Seconds())
	if expired {
		maxAge = 0
	}
	cookie := dashboardSessionCookie + "=" + token + "; Path=/; HttpOnly; SameSite=Lax; Max-Age=" + strconv.Itoa(maxAge)
	if e.cfg.Auth.Dashboard.SecureCookie {
		cookie += "; Secure"
	}
	return cookie
}

func dashboardReservedGroup(group string) bool {
	switch strings.ToLower(strings.TrimSpace(group)) {
	case "api", "healthz", "readyz", "docs", "openapi.json", "openapi.yaml", "assets", "login", "logout":
		return true
	default:
		return false
	}
}
