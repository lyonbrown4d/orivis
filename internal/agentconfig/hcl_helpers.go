package config

import (
	"strings"
	"time"

	"github.com/samber/mo"
)

func setOptional[T any](values map[string]any, key string, value *T) {
	mo.PointerToOption(value).ForEach(func(value T) {
		setValue(values, key, value)
	})
}

func parseAgentHCLDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, wrapErrorf(err, "parse agent HCL duration %q", value)
	}
	return duration, nil
}

func setString(values map[string]any, key, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		setValue(values, key, value)
	}
}

func setValue(values map[string]any, key string, value any) {
	parts := strings.Split(key, ".")
	cursor := values
	for _, part := range parts[:len(parts)-1] {
		next, ok := cursor[part].(map[string]any)
		if !ok {
			next = make(map[string]any)
			cursor[part] = next
		}
		cursor = next
	}
	cursor[parts[len(parts)-1]] = value
}
