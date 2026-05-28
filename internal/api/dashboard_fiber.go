package api

import (
	"context"
	"fmt"
	"time"

	"github.com/arcgolabs/authx"
	"github.com/gofiber/fiber/v3"
	cachex "github.com/lyonbrown4d/orivis/internal/cache"
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/lyonbrown4d/orivis/internal/store"
)

func newDashboardEndpoint(cfg config.Config, storage *store.Store, auth *authx.Engine, cacheStore cachex.Store) *dashboardEndpoint {
	return &dashboardEndpoint{
		cfg:         cfg,
		store:       storage,
		auth:        auth,
		cache:       cacheStore,
		snapshotTTL: dashboardSnapshotCacheTTL(cfg),
	}
}

func (s *Server) registerDashboardRoutes() {
	endpoint := newDashboardEndpoint(s.cfg, s.store, s.auth, s.cache)
	s.app.Post("/api/auth/login", endpoint.fiberLogin)
	s.app.Post("/api/auth/logout", endpoint.fiberLogout)
	s.app.Get("/api/auth/me", endpoint.fiberAuthMe)
	s.app.Get("/api/dashboard/snapshot", endpoint.fiberDashboardSnapshot)
	endpoint.registerTemplateRoutes(s.app)
}

func dashboardSnapshotCacheTTL(cfg config.Config) time.Duration {
	ttl, err := time.ParseDuration(cfg.Dashboard.SnapshotTTL)
	if err != nil || ttl < 0 {
		return time.Second
	}
	return ttl
}

func (e *dashboardEndpoint) fiberLogin(ctx fiber.Ctx) error {
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
	if err := ctx.Bind().Body(&payload); err != nil {
		return fiber.ErrBadRequest
	}
	token, err := e.loginDashboard(ctx.Context(), payload.Username, payload.Password)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	ctx.Set(fiber.HeaderSetCookie, e.dashboardJWTSetCookie(token, false))
	if err := ctx.JSON(fiber.Map{"ok": true}); err != nil {
		return fmt.Errorf("write dashboard login response: %w", err)
	}
	return nil
}

func (e *dashboardEndpoint) fiberLogout(ctx fiber.Ctx) error {
	ctx.Set(fiber.HeaderSetCookie, e.dashboardJWTSetCookie("", true))
	if err := ctx.JSON(fiber.Map{"ok": true}); err != nil {
		return fmt.Errorf("write dashboard logout response: %w", err)
	}
	return nil
}

func (e *dashboardEndpoint) fiberAuthMe(ctx fiber.Ctx) error {
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

func (e *dashboardEndpoint) fiberDashboardSnapshot(ctx fiber.Ctx) error {
	if !e.authenticateDashboardJWT(ctx.Context(), ctx.Cookies(dashboardAuthCookie)) {
		return fiber.ErrUnauthorized
	}
	out, err := e.dashboardSnapshotResponse(ctx.Context(), ctx.Query("group"))
	if err != nil {
		return err
	}
	etag, err := dashboardSnapshotETag(out)
	if err != nil {
		return fmt.Errorf("build dashboard snapshot etag: %w", err)
	}
	ctx.Set(fiber.HeaderETag, etag)
	ctx.Set(fiber.HeaderCacheControl, "private, must-revalidate")
	if dashboardETagMatches(ctx.Get(fiber.HeaderIfNoneMatch), etag) {
		if err := ctx.SendStatus(fiber.StatusNotModified); err != nil {
			return fmt.Errorf("write dashboard snapshot not modified response: %w", err)
		}
		return nil
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
