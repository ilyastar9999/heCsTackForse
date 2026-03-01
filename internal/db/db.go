package db

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func New(dsn string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}
	d := &DB{sqlDB}
	if err := d.Migrate(); err != nil {
		return nil, err
	}
	return d, nil
}
