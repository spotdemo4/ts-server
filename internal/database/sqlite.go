package database

import (
	"github.com/stephenafamo/bob"
	_ "modernc.org/sqlite" // Sqlite
)

func New(dsn string) (*bob.DB, error) {
	// Open db
	db, err := bob.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	return &db, nil
}
