package store

import (
	"context"
	"errors"
	"strings"

	repository "github.com/arcgolabs/dbx/repository"
	"github.com/samber/lo"
)

func ensureCodeEntity(
	ctx context.Context,
	code,
	idPrefix,
	entityName string,
	ids IDGenerator,
	find func(context.Context, string) (string, error),
	insert func(context.Context, string, string) error,
) (string, error) {
	id, err := find(ctx, code)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return "", wrapErrorf(err, "find %s", entityName)
	}

	id, err = ids.NewID(ctx, idPrefix)
	if err != nil {
		return "", wrapErrorf(err, "generate %s id", entityName)
	}
	if err := insert(ctx, id, code); err != nil {
		if !isCodeEntityConflict(err) {
			return "", err
		}
		existingID, findErr := find(ctx, code)
		if findErr == nil {
			return existingID, nil
		}
		if errors.Is(findErr, repository.ErrNotFound) {
			return "", err
		}
		return "", wrapErrorf(findErr, "resolve %s conflict", entityName)
	}
	return id, nil
}

func isCodeEntityConflict(err error) bool {
	if errors.Is(err, repository.ErrConflict) {
		return true
	}
	message := err.Error()
	return lo.ContainsBy([]string{"UNIQUE constraint failed", "duplicate key"}, func(pattern string) bool {
		return strings.Contains(message, pattern)
	})
}
