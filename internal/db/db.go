package db

import (
	"database/sql"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// DB wraps *sql.DB and adds:
//   - driver-aware placeholder rewriting (? → $N for PostgreSQL)
//   - InsertIgnore / InsertGetID helpers for cross-database compatibility
type DB struct {
	*sql.DB
	driver string // "sqlite" or "postgres"
}

// New opens a database connection for the given driver ("sqlite" or "postgres")
// and runs all schema migrations.
func New(driver, dsn string) (*DB, error) {
	driverName := "sqlite"
	if driver == "postgres" || driver == "postgresql" {
		driverName = "postgres"
	}
	sqlDB, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}
	// SQLite: cap writers to avoid SQLITE_BUSY; Postgres: use connection pool defaults.
	if driverName == "sqlite" {
		sqlDB.SetMaxOpenConns(10)
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}
	d := &DB{DB: sqlDB, driver: driverName}
	if err := d.Migrate(); err != nil {
		return nil, err
	}
	return d, nil
}

// IsPostgres returns true when the backing database is PostgreSQL.
func (d *DB) IsPostgres() bool { return d.driver == "postgres" }

// rewrite replaces ? placeholders with $1, $2, … for PostgreSQL.
// For SQLite it is a no-op.
func (d *DB) rewrite(query string) string {
	if d.driver != "postgres" {
		return query
	}
	n := 0
	var buf strings.Builder
	buf.Grow(len(query) + 16)
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			n++
			buf.WriteByte('$')
			buf.WriteString(strconv.Itoa(n))
		} else {
			buf.WriteByte(query[i])
		}
	}
	return buf.String()
}

// Exec rewrites placeholders then delegates to *sql.DB.
func (d *DB) Exec(query string, args ...any) (sql.Result, error) {
	return d.DB.Exec(d.rewrite(query), args...)
}

// Query rewrites placeholders then delegates to *sql.DB.
func (d *DB) Query(query string, args ...any) (*sql.Rows, error) {
	return d.DB.Query(d.rewrite(query), args...)
}

// QueryRow rewrites placeholders then delegates to *sql.DB.
func (d *DB) QueryRow(query string, args ...any) *sql.Row {
	return d.DB.QueryRow(d.rewrite(query), args...)
}

// Prepare rewrites placeholders then delegates to *sql.DB.
func (d *DB) Prepare(query string) (*sql.Stmt, error) {
	return d.DB.Prepare(d.rewrite(query))
}

// InsertIgnore builds a duplicate-key-ignoring INSERT statement.
// Pass the query as:  "INSERT INTO tbl (col) VALUES (?)"
//
//	SQLite  →  INSERT OR IGNORE INTO tbl (col) VALUES (?)
//	Postgres →  INSERT INTO tbl (col) VALUES ($1)  ON CONFLICT DO NOTHING
func (d *DB) InsertIgnore(query string) string {
	if d.driver == "postgres" {
		return query + " ON CONFLICT DO NOTHING"
	}
	return strings.Replace(query, "INSERT INTO ", "INSERT OR IGNORE INTO ", 1)
}

// InsertGetID executes an INSERT and returns the auto-generated row ID.
//
//	SQLite  — uses Exec + LastInsertId
//	Postgres — appends RETURNING id and scans the result
func (d *DB) InsertGetID(query string, args ...any) (int64, error) {
	if d.driver == "postgres" {
		var id int64
		// d.QueryRow already calls rewrite, so ? → $N conversion is handled.
		err := d.QueryRow(query+" RETURNING id", args...).Scan(&id)
		return id, err
	}
	res, err := d.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

