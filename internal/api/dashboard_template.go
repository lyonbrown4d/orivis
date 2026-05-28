package api

import (
	"bytes"
	"html/template"
	"io/fs"
	"path"
	"strings"

	"github.com/gofiber/fiber/v3"
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
	staticFS, err := fs.Sub(ui.Static, "static")
	if err != nil {
		return nil, oops.Wrapf(err, "load dashboard static filesystem")
	}
	return &dashboardTemplateRenderer{
		dashboard: dashboard,
		status:    status,
		login:     login,
		static:    staticFS,
	}, nil
}

func (e *dashboardEndpoint) registerTemplateRoutes(app *fiber.App) {
	renderer, err := newDashboardTemplateRenderer()
	if err != nil {
		app.Get("/", unavailableDashboardTemplate(err))
		app.Get(dashboardRoute, unavailableDashboardTemplate(err))
		app.Get("/:group", unavailableDashboardTemplate(err))
		return
	}

	app.Use("/ui/static", renderer.staticAsset)
	app.Get(loginRoute, e.loginPage(renderer))
	app.Post(loginRoute, e.loginSubmit(renderer))
	app.Get(logoutRoute, e.logoutPage)
	app.Get("/", e.dashboardPage(renderer))
	app.Get(dashboardRoute, e.dashboardPage(renderer))
	app.Get(statusRoute, e.statusPage(renderer))
	app.Get("/:group", e.statusPage(renderer))
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

	name := strings.TrimPrefix(path.Clean(rawName), "/ui/static/")
	name = strings.TrimPrefix(name, "ui/static/")
	name = strings.TrimPrefix(name, "static/")
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
			if err := ctx.Redirect().Status(fiber.StatusFound).To(loginRoute); err != nil {
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

func (e *dashboardEndpoint) loginPage(renderer *dashboardTemplateRenderer) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		if !e.cfg.Auth.Dashboard.Enabled || e.dashboardAuthenticated(ctx) {
			if err := ctx.Redirect().Status(fiber.StatusFound).To(dashboardRoute); err != nil {
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
		if err := ctx.Redirect().Status(fiber.StatusFound).To(dashboardRoute); err != nil {
			return oops.Wrapf(err, "redirect dashboard after login")
		}
		return nil
	}
}

func (e *dashboardEndpoint) logoutPage(ctx fiber.Ctx) error {
	ctx.Set(fiber.HeaderSetCookie, e.dashboardJWTSetCookie("", true))
	if err := ctx.Redirect().Status(fiber.StatusFound).To(loginRoute); err != nil {
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
	case "", "api", "assets", "dashboard", "favicon.ico", "healthz", "login", "logout", "metrics", "status", "ui":
		return true
	default:
		return false
	}
}
