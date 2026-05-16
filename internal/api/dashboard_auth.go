package api

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/arcgolabs/authx"
	"github.com/danielgtaylor/huma/v2"
	"github.com/lyonbrown4d/orivis/internal/security"
)

func (e *dashboardEndpoint) loginDashboard(ctx context.Context, username, password string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return "", fmt.Errorf("%w: username and password are required", errDashboardLoginFailed)
	}
	if strings.TrimSpace(e.cfg.Auth.Dashboard.Username) == "" || e.cfg.Auth.Dashboard.Password == "" {
		return "", huma.Error500InternalServerError("dashboard auth is enabled but credentials are not configured")
	}
	if e.auth == nil {
		return "", huma.Error500InternalServerError("dashboard auth engine is not available")
	}
	result, err := e.auth.Check(ctx, security.DashboardCredential{
		Username: username,
		Password: password,
	})
	if err != nil {
		return "", fmt.Errorf("%w: %w", errDashboardLoginFailed, err)
	}
	if !dashboardPrincipalAllowed(ctx, e.auth, result.Principal) {
		return "", fmt.Errorf("%w: dashboard role is required", errDashboardLoginFailed)
	}
	token, err := e.createDashboardJWT(username)
	if err != nil {
		return "", huma.Error500InternalServerError("create dashboard JWT", err)
	}
	return token, nil
}

func dashboardPrincipalAllowed(ctx context.Context, engine *authx.Engine, principal any) bool {
	if engine == nil {
		return false
	}
	decision, err := engine.Can(ctx, authx.AuthorizationModel{
		Principal: principal,
		Action:    "read",
		Resource:  "dashboard",
	})
	return err == nil && decision.Allowed
}

var errDashboardLoginFailed = errors.New("dashboard login failed")
