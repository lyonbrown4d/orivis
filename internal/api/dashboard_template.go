package api

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/gofiber/template/html/v2"
)

var (
	//go:embed "templates/*.tmpl" "locales/*.json"
	dashboardTemplateFS embed.FS

	dashboardTemplateEngine = func() *html.Engine {
		templateFS, err := fs.Sub(dashboardTemplateFS, "templates")
		if err != nil {
			panic(fmt.Errorf("load dashboard templates: %w", err))
		}

		engine := html.NewFileSystem(http.FS(templateFS), ".tmpl")
		engine.AddFunc("statusClass", dashboardStatusClass)
		engine.AddFunc("duration", dashboardDuration)
		engine.AddFunc("join", dashboardJoin)
		engine.AddFunc("since", dashboardSince)
		engine.AddFunc("groupName", dashboardGroupName)
		engine.AddFunc("groupPath", dashboardGroupPath)
		return engine
	}()
)

func newDashboardViews() *html.Engine {
	return dashboardTemplateEngine
}

func dashboardTemplate() *html.Engine {
	return dashboardTemplateEngine
}
