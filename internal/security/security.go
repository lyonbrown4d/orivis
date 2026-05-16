package security

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"strings"

	"github.com/arcgolabs/authx"
	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/observabilityx"
	"github.com/lyonbrown4d/orivis/internal/serverconfig"
)

type DashboardCredential struct {
	Username string
	Password string
}

func NewEngine(cfg config.Config, logger *slog.Logger, obs observabilityx.Observability) *authx.Engine {
	obs = observabilityx.Normalize(obs, logger)

	engine := authx.NewEngine(
		authx.WithLogger(obs.Logger()),
		authx.WithAuthorizer(authx.RequireAnyRole("admin", "operator", "viewer")),
	)
	if err := authx.RegisterProvider(engine, newDashboardProvider(cfg)); err != nil {
		obs.Logger().Error("register dashboard auth provider failed", "error", err)
	}
	return engine
}

func newDashboardProvider(cfg config.Config) authx.AuthenticationProvider {
	return authx.NewAuthenticationProviderFunc[DashboardCredential](
		func(_ context.Context, credential DashboardCredential) (authx.AuthenticationResult, error) {
			username := strings.TrimSpace(cfg.Auth.Dashboard.Username)
			password := cfg.Auth.Dashboard.Password
			if username == "" || password == "" {
				return authx.AuthenticationResult{}, authx.NewError(
					authx.ErrorCodeAuthenticationProviderNotConfigured,
					"dashboard auth credentials are not configured",
				)
			}
			if !dashboardCredentialMatches(credential, username, password) {
				return authx.AuthenticationResult{}, authx.NewError(
					authx.ErrorCodeUnauthenticated,
					"invalid dashboard credentials",
				)
			}
			return authx.AuthenticationResult{
				Principal: authx.Principal{
					ID:    username,
					Roles: collectionlist.NewList("admin"),
				},
			}, nil
		},
	)
}

func dashboardCredentialMatches(credential DashboardCredential, username, password string) bool {
	usernameOK := subtle.ConstantTimeCompare([]byte(strings.TrimSpace(credential.Username)), []byte(username)) == 1
	passwordOK := subtle.ConstantTimeCompare([]byte(credential.Password), []byte(password)) == 1
	return usernameOK && passwordOK
}
