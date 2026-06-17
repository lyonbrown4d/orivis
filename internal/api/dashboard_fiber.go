package api

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
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
	if basePath := httpBasePath(s.cfg); basePath != "" {
		endpoint.registerDashboardRoutes(s.app.Group(basePath))
	}
	endpoint.registerDashboardRoutes(s.app)
}

func (e *dashboardEndpoint) registerDashboardRoutes(router fiber.Router) {
	router.Post("/api/auth/login", e.fiberLogin)
	router.Post("/api/auth/logout", e.fiberLogout)
	router.Get("/api/auth/me", e.fiberAuthMe)
	router.Get("/api/dashboard/snapshot", e.fiberDashboardSnapshot)
	router.Get("/api/dashboard/monitor/:id", e.fiberDashboardMonitorDetail)
	e.registerTemplateRoutes(router)
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

func (e *dashboardEndpoint) fiberDashboardMonitorDetail(ctx fiber.Ctx) error {
	if !e.authenticateDashboardJWT(ctx.Context(), ctx.Cookies(dashboardAuthCookie)) {
		return fiber.ErrUnauthorized
	}
	monitorID := strings.TrimSpace(ctx.Params("id"))
	if monitorID == "" {
		return fiber.ErrBadRequest
	}
	resultLimit, err := parseDashboardQueryLimit(ctx.Query("results"), 50)
	if err != nil {
		return fiber.ErrBadRequest
	}
	notificationLimit, err := parseDashboardQueryLimit(ctx.Query("notifications"), 20)
	if err != nil {
		return fiber.ErrBadRequest
	}
	out, err := e.store.DashboardMonitorDetail(ctx.Context(), monitorID, resultLimit, notificationLimit)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			return fiber.ErrNotFound
		case errors.Is(err, store.ErrInvalidInput):
			return fiber.ErrBadRequest
		default:
			return fmt.Errorf("load dashboard monitor detail: %w", err)
		}
	}
	response := newDashboardMonitorDetailResponse(out)
	if err := ctx.JSON(response); err != nil {
		return fmt.Errorf("write dashboard monitor detail response: %w", err)
	}
	return nil
}

func parseDashboardQueryLimit(raw string, defaultValue int) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse query limit: %w", err)
	}
	if parsed <= 0 {
		return defaultValue, nil
	}
	return parsed, nil
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
