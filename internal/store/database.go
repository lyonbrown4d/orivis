package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/arcgolabs/dbx"
	"github.com/arcgolabs/dbx/dialect"
	"github.com/arcgolabs/dbx/dialect/mysql"
	"github.com/arcgolabs/dbx/dialect/postgres"
	"github.com/arcgolabs/dbx/dialect/sqlite"
	_ "github.com/go-sql-driver/mysql" // register mysql database driver
	_ "github.com/jackc/pgx/v5/stdlib" // register postgres database driver
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	_ "modernc.org/sqlite" // register sqlite database driver
)

func OpenDB(cfg config.Config, logger *slog.Logger) (*dbx.DB, error) {
	driver := normalizeDBDriver(cfg.DB.Driver)
	if driver == "" {
		cfg.DB.Driver = "sqlite"
		driver = "sqlite"
	}
	switch driver {
	case "sqlite", "sqlite3":
		if strings.TrimSpace(cfg.DB.DSN) == "" {
			cfg.DB.DSN = config.DefaultSQLiteDSN
		}
		return openDatabase(cfg, logger, "sqlite", sqlite.New(), true)
	case "pgx":
		return openDatabase(cfg, logger, "pgx", postgres.New(), false)
	case "mysql":
		return openDatabase(cfg, logger, "mysql", mysql.New(), false)
	default:
		return nil, newErrorf("unsupported database driver %q: supported drivers are sqlite, mysql, pgx", cfg.DB.Driver)
	}
}

func openDatabase(cfg config.Config, logger *slog.Logger, driver string, d dialect.Dialect, forSQLite bool) (*dbx.DB, error) {
	dsn := normalizeDatabaseDSN(driver, cfg.DB.DSN)
	if strings.TrimSpace(dsn) == "" {
		return nil, wrapError(ErrInvalidInput, "db.dsn is required")
	}

	database, err := dbx.Open(
		dbx.WithDriver(driver),
		dbx.WithDSN(dsn),
		dbx.WithDialect(d),
		dbx.ApplyOptions(
			dbx.WithLogger(logger),
			dbx.WithDebug(cfg.App.Env != "production"),
		),
	)
	if err != nil {
		return nil, wrapError(err, "open database store")
	}

	configureConnectionPool(database, cfg)
	if forSQLite {
		configureSQLiteConnection(database, cfg.DB.DSN)
		if err := configureSQLitePragmas(context.Background(), database, cfg); err != nil {
			if closeErr := database.Close(); closeErr != nil {
				return nil, errors.Join(err, wrapError(closeErr, "close database store"))
			}
			return nil, err
		}
	}

	return database, nil
}

func configureSQLiteConnection(database *dbx.DB, dsn string) {
	if database == nil || database.SQLDB() == nil {
		return
	}
	database.SQLDB().SetConnMaxIdleTime(5 * time.Minute)
	if isSQLiteMemoryDSN(dsn) {
		database.SQLDB().SetMaxOpenConns(1)
		database.SQLDB().SetMaxIdleConns(1)
		return
	}
	database.SQLDB().SetMaxOpenConns(4)
	database.SQLDB().SetMaxIdleConns(4)
}

func configureConnectionPool(database *dbx.DB, cfg config.Config) {
	if database == nil || database.SQLDB() == nil || cfg.DB.MaxOpenConns <= 0 {
		return
	}
	database.SQLDB().SetMaxOpenConns(cfg.DB.MaxOpenConns)
	database.SQLDB().SetMaxIdleConns(cfg.DB.MaxOpenConns)
}

func configureSQLitePragmas(ctx context.Context, database *dbx.DB, cfg config.Config) error {
	if database == nil {
		return nil
	}
	if err := execSQLitePragma(ctx, database, "PRAGMA foreign_keys = ON"); err != nil {
		return err
	}
	if err := execSQLitePragma(ctx, database, "PRAGMA busy_timeout = "+sqliteBusyTimeoutMillis(cfg.DB.BusyTimeout)); err != nil {
		return err
	}
	if isSQLiteMemoryDSN(cfg.DB.DSN) {
		return nil
	}
	if err := execSQLitePragma(ctx, database, "PRAGMA journal_mode = WAL"); err != nil {
		return err
	}
	return execSQLitePragma(ctx, database, "PRAGMA synchronous = NORMAL")
}

func execSQLitePragma(ctx context.Context, database *dbx.DB, query string) error {
	if _, err := database.ExecContext(ctx, query); err != nil {
		return wrapErrorf(err, "configure sqlite pragma %q", query)
	}
	return nil
}

func sqliteBusyTimeoutMillis(value string) string {
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || duration <= 0 {
		duration = 5 * time.Second
	}
	return strconv.FormatInt(duration.Milliseconds(), 10)
}

func isSQLiteMemoryDSN(dsn string) bool {
	normalizedDSN := strings.ToLower(dsn)
	return strings.Contains(normalizedDSN, "mode=memory") || strings.Contains(normalizedDSN, ":memory:")
}

func normalizeDatabaseDSN(driver, rawDSN string) string {
	normalizedDriver := normalizeDBDriver(driver)
	normalizedDSN := strings.TrimSpace(rawDSN)
	if normalizedDriver != "mysql" {
		return normalizedDSN
	}
	if normalizedDSN == "" || !strings.HasPrefix(normalizedDSN, "mysql://") {
		return normalizedDSN
	}

	parsed, err := url.Parse(normalizedDSN)
	if err != nil {
		return normalizedDSN
	}

	password, _ := parsed.User.Password()
	query := strings.TrimSpace(parsed.RawQuery)
	user := parsed.User.Username()
	addr := parsed.Host
	if addr == "" {
		addr = "127.0.0.1:3306"
	}
	dbName := strings.TrimPrefix(parsed.Path, "/")

	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s", user, password, addr, dbName)
	if query != "" {
		dsn += "?" + query
	}
	return dsn
}

// NormalizeDatabaseDSN returns the driver-specific normalized DSN used by store.OpenDB.
func NormalizeDatabaseDSN(driver, rawDSN string) string {
	return normalizeDatabaseDSN(driver, rawDSN)
}

func normalizeDBDriver(driver string) string {
	return strings.ToLower(strings.TrimSpace(driver))
}
