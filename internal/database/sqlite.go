package database

import (
	"errors"
	"strings"

	"github.com/stephenafamo/bob"
	_ "modernc.org/sqlite" // Sqlite
)

func New(dsn string) (*bob.DB, error) {
	// Validate dsn
	sp := strings.Split(dsn, ":")
	if len(sp) != 2 {
		return nil, errors.New("invalid dsn")
	}

	// Open db
	db, err := bob.Open("sqlite", sp[1])
	if err != nil {
		return nil, err
	}

	return &db, nil
}
