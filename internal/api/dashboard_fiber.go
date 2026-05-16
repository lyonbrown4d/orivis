package api

import (
	"context"
	"fmt"

	"github.com/arcgolabs/authx"
	"github.com/gofiber/fiber/v2"
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/lyonbrown4d/orivis/internal/store"
)

func newDashboardEndpoint(cfg config.Config, storage *store.Store, auth *authx.Engine) *dashboardEndpoint {
	return &dashboardEndpoint{
		cfg:   cfg,
		store: storage,
		auth:  auth,
	}
}

func (s *Server) registerDashboardRoutes() {
	endpoint := newDashboardEndpoint(s.cfg, s.store, s.auth)
	s.app.Post("/api/auth/login", endpoint.fiberLogin)
	s.app.Post("/api/auth/logout", endpoint.fiberLogout)
	s.app.Get("/api/auth/me", endpoint.fiberAuthMe)
	s.app.Get("/api/dashboard/snapshot", endpoint.fiberDashboardSnapshot)
}

func (e *dashboardEndpoint) fiberLogin(ctx *fiber.Ctx) error {
	if !e.cfg.Auth.Dashboard.Enabled {
		if err := ctx.JSON(fiber.Map{"ok": true}); err != nil {
			return fmt.Errorf("write dashboard login response: %w", err)
		}
		return nil
	}

	var payload struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := ctx.BodyParser(&payload); err != nil {
		return fiber.ErrBadRequest
	}
	token, err := e.loginDashboard(ctx.UserContext(), payload.Username, payload.Password)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	ctx.Set(fiber.HeaderSetCookie, e.dashboardJWTSetCookie(token, false))
	if err := ctx.JSON(fiber.Map{"ok": true}); err != nil {
		return fmt.Errorf("write dashboard login response: %w", err)
	}
	return nil
}

func (e *dashboardEndpoint) fiberLogout(ctx *fiber.Ctx) error {
	ctx.Set(fiber.HeaderSetCookie, e.dashboardJWTSetCookie("", true))
	if err := ctx.JSON(fiber.Map{"ok": true}); err != nil {
		return fmt.Errorf("write dashboard logout response: %w", err)
	}
	return nil
}

func (e *dashboardEndpoint) fiberAuthMe(ctx *fiber.Ctx) error {
	if !e.cfg.Auth.Dashboard.Enabled {
		if err := ctx.JSON(fiber.Map{"authenticated": true, "username": "public"}); err != nil {
			return fmt.Errorf("write dashboard auth response: %w", err)
		}
		return nil
	}
	claims, ok := e.dashboardClaims(ctx.Cookies(dashboardAuthCookie))
	if !ok {
		if err := ctx.JSON(fiber.Map{"authenticated": false}); err != nil {
			return fmt.Errorf("write dashboard auth response: %w", err)
		}
		return nil
	}
	if err := ctx.JSON(fiber.Map{"authenticated": true, "username": claims.Subject}); err != nil {
		return fmt.Errorf("write dashboard auth response: %w", err)
	}
	return nil
}

func (e *dashboardEndpoint) fiberDashboardSnapshot(ctx *fiber.Ctx) error {
	if !e.authenticateDashboardJWT(ctx.UserContext(), ctx.Cookies(dashboardAuthCookie)) {
		return fiber.ErrUnauthorized
	}
	out, err := e.dashboardSnapshotResponse(ctx.UserContext(), ctx.Query("group"))
	if err != nil {
		return err
	}
	if err := ctx.JSON(out); err != nil {
		return fmt.Errorf("write dashboard snapshot response: %w", err)
	}
	return nil
}

func (e *dashboardEndpoint) dashboardClaims(token string) (dashboardJWTClaims, bool) {
	if !e.cfg.Auth.Dashboard.Enabled {
		return dashboardJWTClaims{Subject: "public"}, true
	}
	secret, err := e.dashboardJWTSecret()
	if err != nil {
		return dashboardJWTClaims{}, false
	}
	claims, err := verifyDashboardJWT(context.Background(), token, []byte(secret))
	if err != nil {
		return dashboardJWTClaims{}, false
	}
	return claims, true
}
