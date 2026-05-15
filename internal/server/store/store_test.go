package store

import "testing"

func TestResolveDialectSQLite(t *testing.T) {
	dialect, driver, err := resolveDialect("sqlite")
	if err != nil {
		t.Fatalf("expected sqlite dialect: %v", err)
	}
	if driver != "sqlite" {
		t.Fatalf("expected sqlite driver, got %q", driver)
	}
	if dialect.Name() != "sqlite" {
		t.Fatalf("expected sqlite dialect, got %q", dialect.Name())
	}
}

func TestResolveDialectUnsupported(t *testing.T) {
	_, _, err := resolveDialect("postgres")
	if err == nil {
		t.Fatal("expected unsupported driver error")
	}
}
