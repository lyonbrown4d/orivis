package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func (s *Server) registerSPARoutes() {
	if !s.cfg.Web.Enabled {
		return
	}

	root := filepath.Clean(strings.TrimSpace(s.cfg.Web.Root))
	if root == "." {
		s.logger.Error("web root is empty")
		s.registerUnavailableSPA()
		return
	}

	indexHTML, err := loadSPAIndex(root)
	if err != nil {
		s.logger.Error("web index is not available", "root", root, "error", err)
		s.registerUnavailableSPA()
		return
	}

	assetsRoot := filepath.Join(root, "assets")
	if stat, err := os.Stat(assetsRoot); err == nil && stat.IsDir() {
		s.app.Static("/assets", assetsRoot, fiber.Static{Index: ""})
	}
	s.app.Get("/*", func(ctx *fiber.Ctx) error {
		if skipBackendPath(ctx) || skipAssetPath(ctx) {
			return fiber.ErrNotFound
		}
		if !isSPAMethod(ctx.Method()) {
			return fiber.ErrNotFound
		}
		ctx.Type("html")
		return ctx.Send(indexHTML)
	})
}

func loadSPAIndex(root string) (indexHTML []byte, err error) {
	file, err := http.Dir(root).Open("index.html")
	if err != nil {
		return nil, fmt.Errorf("open index file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close index file: %w", closeErr)
		}
	}()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat index file: %w", err)
	}
	if stat.IsDir() {
		return nil, errors.New("index file is a directory")
	}

	indexHTML, err = io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read index file: %w", err)
	}
	return indexHTML, nil
}

func (s *Server) registerUnavailableSPA() {
	s.app.Get("/*", func(ctx *fiber.Ctx) error {
		if skipBackendPath(ctx) {
			return fiber.ErrNotFound
		}
		return fiber.ErrServiceUnavailable
	})
}

func skipBackendPath(ctx *fiber.Ctx) bool {
	path := ctx.Path()
	return path == "/api" ||
		strings.HasPrefix(path, "/api/") ||
		path == "/healthz" ||
		path == "/readyz" ||
		path == "/docs" ||
		strings.HasPrefix(path, "/docs/") ||
		path == "/openapi" ||
		strings.HasPrefix(path, "/openapi/") ||
		path == "/metrics"
}

func skipAssetPath(ctx *fiber.Ctx) bool {
	return ctx.Path() == "/assets" || strings.HasPrefix(ctx.Path(), "/assets/")
}

func isSPAMethod(method string) bool {
	return method == fiber.MethodGet || method == fiber.MethodHead
}
