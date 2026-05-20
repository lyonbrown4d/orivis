package migrations

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

//go:embed sqlite/*.sql
var files embed.FS

type File struct {
	Version string
	Name    string
	SQL     string
}

func SQLite() ([]File, error) {
	return readDir("sqlite")
}

func All() ([]File, error) {
	return SQLite()
}

func readDir(dir string) ([]File, error) {
	entries, err := fs.ReadDir(files, dir)
	if err != nil {
		return nil, fmt.Errorf("read migration dir %s: %w", dir, err)
	}

	out := make([]File, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		name := entry.Name()
		content, err := files.ReadFile(path.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("read migration file %s: %w", name, err)
		}

		version := strings.TrimSuffix(name, ".sql")
		out = append(out, File{
			Version: version,
			Name:    name,
			SQL:     string(content),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out, nil
}
