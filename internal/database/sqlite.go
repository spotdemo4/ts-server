package database

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/spotdemo4/ts-server/internal/sqlc"
	_ "modernc.org/sqlite" // Sqlite
)

func New(dsn string) (*sqlc.Queries, *sql.DB, error) {
	// Validate dsn
	sp := strings.Split(dsn, ":")
	if len(sp) != 2 {
		return nil, nil, errors.New("invalid dsn")
	}

	// Open db
	db, err := sql.Open("sqlite", sp[1])
	if err != nil {
		return nil, nil, err
	}

	// Create new sqlc connection
	sqlc := sqlc.New(db)

	return sqlc, db, nil
}
