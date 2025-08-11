package database

import (
	"strings"

	"github.com/stephenafamo/bob"
	_ "modernc.org/sqlite" // Sqlite
)

func New(dsn string) (*bob.DB, error) {
	// Format dsn for sqlite
	if strings.HasPrefix(dsn, "sqlite:") {
		dsn = strings.TrimPrefix(dsn, "sqlite:")
		if !strings.HasPrefix(dsn, "/") {
			dsn = "/" + dsn // Ensure absolute path for sqlite
		}
	}

	// Open db
	db, err := bob.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	return &db, nil
}
