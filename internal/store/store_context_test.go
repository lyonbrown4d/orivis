package store_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/lyonbrown4d/orivis/internal/store"
)

func TestOpenWithContextCancelsSQLitePragmaInitialization(t *testing.T) {
	t.Parallel()

	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"
	cfg.DB.DSN = filepath.Join(t.TempDir(), "orivis-cancel-pragmas.db")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.OpenWithContext(ctx, cfg, testLogger())
	if err == nil {
		t.Fatal("expected context cancellation to fail store open")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled context error, got %v", err)
	}
}

func TestWithContextAPIsRejectNilContext(t *testing.T) {
	t.Parallel()

	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"

	for _, testName := range []string{
		"OpenWithContext",
		"OpenDBWithContext",
		"NewWithContext",
		"NewWithDriverWithContext",
	} {
		t.Run(testName, func(t *testing.T) {
			t.Helper()
			err := runContextAPIForNilContext(testName, cfg, contextFactoryNil)
			if err == nil {
				t.Fatalf("expected nil-context error")
			}
			if !errors.Is(err, store.ErrInvalidInput) {
				t.Fatalf("expected invalid input error, got %v", err)
			}
		})
	}
}

func TestNewWithContextCancelsMigrationInitialization(t *testing.T) {
	t.Parallel()

	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"
	cfg.DB.DSN = filepath.Join(t.TempDir(), "orivis-cancel-migrations.db")

	database, err := store.OpenDBWithContext(context.Background(), cfg, testLogger())
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Fatalf("close database: %v", closeErr)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = store.NewWithContext(ctx, database, nil, nil)
	if err == nil {
		t.Fatal("expected context cancellation to fail store migration")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled context error, got %v", err)
	}
}

func runContextAPIForNilContext(name string, cfg config.Config, contextFactory func() context.Context) error {
	ctx := contextFactory()
	switch name {
	case "OpenWithContext":
		_, err := store.OpenWithContext(ctx, cfg, testLogger())
		return fmt.Errorf("OpenWithContext: %w", err)
	case "OpenDBWithContext":
		_, err := store.OpenDBWithContext(ctx, cfg, testLogger())
		return fmt.Errorf("OpenDBWithContext: %w", err)
	case "NewWithContext":
		_, err := store.NewWithContext(ctx, nil, nil, nil)
		return fmt.Errorf("NewWithContext: %w", err)
	case "NewWithDriverWithContext":
		_, err := store.NewWithDriverWithContext(ctx, nil, "sqlite", nil, nil)
		return fmt.Errorf("NewWithDriverWithContext: %w", err)
	default:
		return fmt.Errorf("unknown context API: %s", name)
	}
}

func contextFactoryNil() context.Context {
	return nil
}
