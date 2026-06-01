package store_test

import (
	"testing"

	"github.com/lyonbrown4d/orivis/internal/store"
)

func TestNormalizeDatabaseDSNForMySQL(t *testing.T) {
	tests := []struct {
		name     string
		driver   string
		input    string
		expected string
	}{
		{
			name:     "mysql url with credentials and database",
			driver:   "mysql",
			input:    "mysql://app@127.0.0.1:3306/orivis?parseTime=true",
			expected: "app:@tcp(127.0.0.1:3306)/orivis?parseTime=true",
		},
		{
			name:     "mysql url without credentials",
			driver:   "mysql",
			input:    "mysql://127.0.0.1:3306/orivis?readTimeout=30s",
			expected: ":@tcp(127.0.0.1:3306)/orivis?readTimeout=30s",
		},
		{
			name:     "non mysql dsn passthrough",
			driver:   "mysql",
			input:    "app_user:secret@tcp(127.0.0.1:3306)/orivis?parseTime=true",
			expected: "app_user:secret@tcp(127.0.0.1:3306)/orivis?parseTime=true",
		},
		{
			name:     "postgres passthrough",
			driver:   "pgx",
			input:    "postgres://127.0.0.1:5432/orivis?sslmode=disable",
			expected: "postgres://127.0.0.1:5432/orivis?sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := store.NormalizeDatabaseDSN(tt.driver, tt.input)
			if got != tt.expected {
				t.Fatalf("normalize database dsn for driver %s: got %q expected %q", tt.driver, got, tt.expected)
			}
		})
	}
}

func TestIsSQLiteMemoryDSN(t *testing.T) {
	tests := []struct {
		name     string
		dsn      string
		expected bool
	}{
		{
			name:     "memory mode in file dsn",
			dsn:      "file:orivis?mode=memory&cache=shared",
			expected: true,
		},
		{
			name:     "raw memory dsn",
			dsn:      ":memory:",
			expected: true,
		},
		{
			name:     "uppercase mode",
			dsn:      "file:orivis?mode=MEMORY",
			expected: true,
		},
		{
			name:     "explicit file memory dsn",
			dsn:      "file::memory:?cache=shared",
			expected: true,
		},
		{
			name:     "file dsn without memory",
			dsn:      "file:/tmp/orivis.db?_journal_mode=WAL",
			expected: false,
		},
		{
			name:     "empty dsn",
			dsn:      "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := store.IsSQLiteMemoryDSN(tt.dsn)
			if got != tt.expected {
				t.Fatalf("isSQLiteMemoryDSN(%q): got %v expected %v", tt.dsn, got, tt.expected)
			}
		})
	}
}
