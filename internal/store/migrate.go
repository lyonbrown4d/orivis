package store

import (
	"context"
	"strings"
	"time"

	"github.com/lyonbrown4d/orivis/migrations"
)

const migrationsTableSQL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TEXT NOT NULL
)`

func (s *Store) Migrate(ctx context.Context) error {
	if s == nil || s.DB == nil {
		return nil
	}

	if _, err := s.DB.ExecContext(ctx, migrationsTableSQL); err != nil {
		return wrapError(err, "ensure migrations table")
	}

	files, err := migrations.All()
	if err != nil {
		return wrapError(err, "load migrations")
	}
	return s.applyPendingMigrations(ctx, files)
}

func (s *Store) applyPendingMigrations(ctx context.Context, files []migrations.File) error {
	for _, file := range files {
		applied, err := s.migrationApplied(ctx, file.Version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := s.applyMigration(ctx, file); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) migrationApplied(ctx context.Context, version string) (bool, error) {
	var count int
	if err := s.DB.QueryRowContext(ctx, "SELECT COUNT(1) FROM schema_migrations WHERE version = ?", version).Scan(&count); err != nil {
		return false, wrapErrorf(err, "check migration %s", version)
	}
	return count > 0, nil
}

func (s *Store) applyMigration(ctx context.Context, file migrations.File) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return wrapErrorf(err, "begin migration %s", file.Version)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if err := tx.RollbackContext(ctx); err != nil {
			return
		}
	}()

	for _, stmt := range splitSQLScript(file.SQL) {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return wrapErrorf(err, "apply migration %s", file.Version)
		}
	}

	if _, err := tx.ExecContext(
		ctx,
		"INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)",
		file.Version,
		formatTime(time.Now().UTC()),
	); err != nil {
		return wrapErrorf(err, "record migration %s", file.Version)
	}

	if err := tx.CommitContext(ctx); err != nil {
		return wrapErrorf(err, "commit migration %s", file.Version)
	}
	committed = true
	return nil
}

func splitSQLScript(script string) []string {
	parts := strings.Split(script, ";")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		stmt := strings.TrimSpace(part)
		if stmt == "" {
			continue
		}
		out = append(out, stmt)
	}
	return out
}
