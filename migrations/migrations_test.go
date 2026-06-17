package migrations_test

import (
	"testing"

	"github.com/lyonbrown4d/orivis/migrations"
)

func TestForDriverLoadsDialectMigrations(t *testing.T) {
	for _, driver := range []string{"sqlite", "mysql", "pgx"} {
		files, err := migrations.ForDriver(driver)
		if err != nil {
			t.Fatalf("load %s migrations: %v", driver, err)
		}
		if len(files) != 7 {
			t.Fatalf("expected seven %s migrations, got %d", driver, len(files))
		}
		if files[0].Name != "001_init.sql" || files[6].Name != "007_probe_result_id.sql" {
			t.Fatalf("unexpected %s migration ordering: %#v", driver, files)
		}
	}
}

func TestForDriverAliases(t *testing.T) {
	files, err := migrations.ForDriver("postgres")
	if err != nil {
		t.Fatalf("load postgres alias migrations: %v", err)
	}
	if len(files) != 7 {
		t.Fatalf("expected postgres alias to load pgx migrations, got %d", len(files))
	}
}
