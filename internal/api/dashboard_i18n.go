package api

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"sync"
)

var (
	dashboardLocaleCatalog  = make(map[string]map[string]string)
	dashboardLocaleLoadOnce sync.Once
	dashboardLocaleLoadErr  error
)

func dashboardLocale(lang string, fallbacks ...string) string {
	if locale := dashboardLocaleValue(lang); locale != "" {
		return locale
	}
	for _, fallback := range fallbacks {
		if locale := dashboardLocaleHeader(fallback); locale != "" {
			return locale
		}
	}
	return "en"
}

func dashboardLocaleHeader(value string) string {
	for item := range strings.SplitSeq(value, ",") {
		token := strings.TrimSpace(strings.SplitN(item, ";", 2)[0])
		if locale := dashboardLocaleValue(token); locale != "" {
			return locale
		}
	}
	return ""
}

func dashboardLocaleValue(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	switch lang {
	case "zh":
		return "zh"
	case "zh-cn":
		return "zh"
	case "zh-hans":
		return "zh"
	case "en":
		return "en"
	case "en-us":
		return "en"
	default:
		return dashboardLocalePrefix(lang)
	}
}

func dashboardLocalePrefix(lang string) string {
	switch {
	case strings.HasPrefix(lang, "zh-"):
		return "zh"
	case strings.HasPrefix(lang, "en-"):
		return "en"
	default:
		return ""
	}
}

func dashboardLangOptions(activeLang string) []dashboardLanguageOption {
	t := dashboardT(activeLang)
	return []dashboardLanguageOption{
		{Code: "en", Label: dashboardText(t, "lang.option.en", "English"), Active: activeLang == "en"},
		{Code: "zh", Label: dashboardText(t, "lang.option.zh", "\u4e2d\u6587"), Active: activeLang == "zh"},
	}
}

func dashboardT(lang string) func(string) string {
	locales, err := dashboardTranslations()
	if err != nil {
		return func(key string) string {
			return key
		}
	}

	values := locales[lang]
	fallback := locales["en"]
	return func(key string) string {
		if text, ok := values[key]; ok {
			return text
		}
		if text, ok := fallback[key]; ok {
			return text
		}
		return key
	}
}

func dashboardTranslations() (map[string]map[string]string, error) {
	dashboardLocaleLoadOnce.Do(func() {
		dashboardLocaleCatalog, dashboardLocaleLoadErr = loadDashboardTranslations()
	})
	return dashboardLocaleCatalog, dashboardLocaleLoadErr
}

func loadDashboardTranslations() (map[string]map[string]string, error) {
	localeFS, err := fs.Sub(dashboardTemplateFS, "locales")
	if err != nil {
		return nil, fmt.Errorf("load dashboard locale filesystem: %w", err)
	}
	entries, err := fs.ReadDir(localeFS, ".")
	if err != nil {
		return nil, fmt.Errorf("read dashboard locale files: %w", err)
	}

	catalog := make(map[string]map[string]string)
	for _, entry := range entries {
		if !dashboardLocaleEntry(entry) {
			continue
		}
		locale, values, err := readDashboardLocale(localeFS, entry.Name())
		if err != nil {
			return nil, err
		}
		catalog[locale] = values
	}
	return catalog, nil
}

func dashboardLocaleEntry(entry fs.DirEntry) bool {
	return !entry.IsDir() && path.Ext(entry.Name()) == ".json"
}

func readDashboardLocale(localeFS fs.FS, name string) (string, map[string]string, error) {
	content, err := fs.ReadFile(localeFS, name)
	if err != nil {
		return "", nil, fmt.Errorf("read locale file %s: %w", name, err)
	}
	values := make(map[string]string)
	if err := json.Unmarshal(content, &values); err != nil {
		return "", nil, fmt.Errorf("parse locale file %s: %w", name, err)
	}
	return strings.TrimSuffix(name, path.Ext(name)), values, nil
}
