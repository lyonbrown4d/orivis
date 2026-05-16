package store

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/arcgolabs/dbx"
	"github.com/arcgolabs/dbx/idgen"
)

var errIDGeneratorUnavailable = errors.New("generate id: dbx id generator is not available")

type IDGenerator interface {
	NewID(ctx context.Context, prefix string) (string, error)
}

type dbxIDGenerator struct {
	database *dbx.DB
}

func NewIDGenerator(database *dbx.DB) IDGenerator {
	return dbxIDGenerator{database: database}
}

func (g dbxIDGenerator) NewID(ctx context.Context, prefix string) (string, error) {
	if g.database == nil || g.database.IDGenerator() == nil {
		return "", errIDGeneratorUnavailable
	}
	raw, err := g.database.IDGenerator().GenerateID(ctx, idgen.Request{Strategy: idgen.StrategySnowflake})
	if err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	id, ok := raw.(int64)
	if !ok {
		return "", fmt.Errorf("generate id: unexpected snowflake id type %T", raw)
	}
	return prefix + "_" + strconv.FormatInt(id, 10), nil
}
