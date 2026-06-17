package api

import (
	"bytes"
	"html/template"
	"io/fs"
	"path"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/lyonbrown4d/orivis/internal/store"
	"github.com/lyonbrown4d/orivis/internal/ui"
	"github.com/samber/oops"
)

func newDashboardTemplateRenderer() (*dashboardTemplateRenderer, error) {
	dashboard, err := ui.ParseTemplate(ui.TemplateDashboard)
	if err != nil {
		return nil, oops.Wrapf(err, "parse dashboard template")
	}
	status, err := ui.ParseTemplate(ui.TemplateStatus)
	if err != nil {
		return nil, oops.Wrapf(err, "parse status template")
	}
	login, err := ui.ParseTemplate(ui.TemplateLogin)
	if err != nil {
		return nil, oops.Wrapf(err, "parse login template")
	}
	monitor, err := ui.ParseTemplate(ui.TemplateMonitorDetail)
	if err != nil {
		return nil, oops.Wrapf(err, "parse monitor detail template")
	}
	staticFS, err := fs.Sub(ui.Static, "static")
	if err != nil {
		return nil, oops.Wrapf(err, "load dashboard static filesystem")
	}
	return &dashboardTemplateRenderer{
		dashboard: dashboard,
		status:    status,
		login:     login,
		monitor:   monitor,
		static:    staticFS,
	}, nil
}

func (e *dashboardEndpoint) registerTemplateRoutes(router fiber.Router) {
	renderer, err := newDashboardTemplateRenderer()
	if err != nil {
		router.Get("/", unavailableDashboardTemplate(err))
		router.Get(dashboardRoute, unavailableDashboardTemplate(err))
		router.Get("/:group", unavailableDashboardTemplate(err))
		router.Get(dashboardMonitorDetailRoute+"/:id", unavailableDashboardTemplate(err))
		router.Get(monitorDetailRoute+"/:id", unavailableDashboardTemplate(err))
		return
	}

	router.Use("/ui/static", renderer.staticAsset)
	router.Get(loginRoute, e.loginPage(renderer))
	router.Post(loginRoute, e.loginSubmit(renderer))
	router.Get(logoutRoute, e.logoutPage)
	router.Get("/", e.dashboardPage(renderer))
	router.Get(dashboardRoute, e.dashboardPage(renderer))
	router.Get(statusRoute, e.statusPage(renderer))
	router.Get("/:group", e.statusPage(renderer))
	router.Get(dashboardMonitorDetailRoute+"/:id", e.dashboardMonitorDetailPage(renderer, false))
	router.Get(monitorDetailRoute+"/:id", e.dashboardMonitorDetailPage(renderer, true))
}

func unavailableDashboardTemplate(err error) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		return oops.Wrapf(err, "render dashboard template")
	}
}

func (r *dashboardTemplateRenderer) staticAsset(ctx fiber.Ctx) error {
	rawName := strings.TrimSpace(ctx.Path())
	if rawName == "" {
		return fiber.ErrNotFound
	}

	name := staticAssetName(rawName)
	if name == "." || strings.HasPrefix(name, "../") || strings.Contains(name, "/../") {
		return fiber.ErrNotFound
	}

	content, err := fs.ReadFile(r.static, name)
	if err != nil {
		return fiber.ErrNotFound
	}
	switch path.Ext(name) {
	case ".css":
		ctx.Set(fiber.HeaderContentType, "text/css; charset=utf-8")
	case ".js":
		ctx.Set(fiber.HeaderContentType, "application/javascript; charset=utf-8")
	default:
		ctx.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)
	}
	if err := ctx.Send(content); err != nil {
		return oops.Wrapf(err, "send dashboard static asset")
	}
	return nil
}

func (e *dashboardEndpoint) dashboardPage(renderer *dashboardTemplateRenderer) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		if !e.dashboardAuthenticated(ctx) {
			if err := ctx.Redirect().Status(fiber.StatusFound).To(prefixedPath(e.cfg, loginRoute)); err != nil {
				return oops.Wrapf(err, "redirect dashboard login")
			}
			return nil
		}
		view, err := e.dashboardView(ctx.Context(), "")
		if err != nil {
			return oops.Wrapf(err, "load dashboard view")
		}
		page := newDashboardTemplatePage(ctx, e, view, false, "")
		return renderer.render(ctx, renderer.dashboard, page)
	}
}

func (e *dashboardEndpoint) statusPage(renderer *dashboardTemplateRenderer) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		group := strings.TrimSpace(ctx.Params("group"))
		if group != "" && dashboardReservedSlug(group) {
			return fiber.ErrNotFound
		}
		view, err := e.dashboardView(ctx.Context(), group)
		if err != nil {
			return oops.Wrapf(err, "load status view")
		}
		page := newDashboardTemplatePage(ctx, e, view, true, "")
		return renderer.render(ctx, renderer.status, page)
	}
}

func (e *dashboardEndpoint) dashboardMonitorDetailPage(renderer *dashboardTemplateRenderer, public bool) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		if err := e.ensureDashboardMonitorDetailAccess(ctx, public); err != nil {
			return err
		}
		detail, err := e.loadDashboardMonitorDetail(ctx)
		if err != nil {
			return err
		}
		page := newDashboardTemplateMonitorDetailPage(ctx, e, detail, public, "")
		return renderer.render(ctx, renderer.monitor, page)
	}
}

func (e *dashboardEndpoint) ensureDashboardMonitorDetailAccess(ctx fiber.Ctx, public bool) error {
	if public || e.dashboardAuthenticated(ctx) {
		return nil
	}
	if err := ctx.Redirect().Status(fiber.StatusFound).To(prefixedPath(e.cfg, loginRoute)); err != nil {
		return oops.Wrapf(err, "redirect dashboard login")
	}
	return nil
}

func (e *dashboardEndpoint) loadDashboardMonitorDetail(ctx fiber.Ctx) (store.DashboardMonitorDetail, error) {
	monitorID := strings.TrimSpace(ctx.Params("id"))
	if monitorID == "" {
		return store.DashboardMonitorDetail{}, fiber.ErrBadRequest
	}
	resultLimit, err := parseDashboardQueryLimit(ctx.Query("results"), 50)
	if err != nil {
		return store.DashboardMonitorDetail{}, fiber.ErrBadRequest
	}
	notificationLimit, err := parseDashboardQueryLimit(ctx.Query("notifications"), 20)
	if err != nil {
		return store.DashboardMonitorDetail{}, fiber.ErrBadRequest
	}
	detail, err := e.store.DashboardMonitorDetail(ctx.Context(), monitorID, resultLimit, notificationLimit)
	if err != nil {
		return store.DashboardMonitorDetail{}, oops.Wrapf(err, "load monitor detail")
	}
	return detail, nil
}

func (e *dashboardEndpoint) loginPage(renderer *dashboardTemplateRenderer) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		if !e.cfg.Auth.Dashboard.Enabled || e.dashboardAuthenticated(ctx) {
			if err := ctx.Redirect().Status(fiber.StatusFound).To(prefixedPath(e.cfg, dashboardRoute)); err != nil {
				return oops.Wrapf(err, "redirect dashboard")
			}
			return nil
		}
		return renderer.render(ctx, renderer.login, newLoginTemplatePage(ctx, e, ""))
	}
}

func (e *dashboardEndpoint) loginSubmit(renderer *dashboardTemplateRenderer) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		token, err := e.loginDashboard(ctx.Context(), ctx.FormValue("username"), ctx.FormValue("password"))
		if err != nil {
			ctx.Status(fiber.StatusUnauthorized)
			return renderer.render(ctx, renderer.login, newLoginTemplatePage(ctx, e, dashboardTemplateTexts(ctx).LoginFailed))
		}
		ctx.Set(fiber.HeaderSetCookie, e.dashboardJWTSetCookie(token, false))
		if err := ctx.Redirect().Status(fiber.StatusFound).To(prefixedPath(e.cfg, dashboardRoute)); err != nil {
			return oops.Wrapf(err, "redirect dashboard after login")
		}
		return nil
	}
}

func (e *dashboardEndpoint) logoutPage(ctx fiber.Ctx) error {
	ctx.Set(fiber.HeaderSetCookie, e.dashboardJWTSetCookie("", true))
	if err := ctx.Redirect().Status(fiber.StatusFound).To(prefixedPath(e.cfg, loginRoute)); err != nil {
		return oops.Wrapf(err, "redirect dashboard logout")
	}
	return nil
}

func (e *dashboardEndpoint) dashboardAuthenticated(ctx fiber.Ctx) bool {
	if !e.cfg.Auth.Dashboard.Enabled {
		return true
	}
	return e.authenticateDashboardJWT(ctx.Context(), ctx.Cookies(dashboardAuthCookie))
}

func (r *dashboardTemplateRenderer) render(ctx fiber.Ctx, tmpl *template.Template, data dashboardTemplatePage) error {
	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		return oops.Wrapf(err, "execute dashboard template")
	}
	ctx.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	if err := ctx.Send(body.Bytes()); err != nil {
		return oops.Wrapf(err, "send dashboard template")
	}
	return nil
}

func dashboardReservedSlug(slug string) bool {
	switch strings.ToLower(strings.TrimSpace(slug)) {
	case "", "api", "assets", "dashboard", "favicon.ico", "login", "logout", "metrics", "status", "ui", monitorDetailSlug:
		return true
	default:
		return false
	}
}
