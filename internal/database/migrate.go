package database

import (
	"embed"
	"log/slog"
	"net/url"

	"github.com/amacneil/dbmate/v2/pkg/dbmate"
	_ "github.com/spotdemo4/dbmate-sqlite-modernc/pkg/driver/sqlite" // Modernc sqlite
)

// Migrate applies database migrations using dbmate.
func Migrate(dsn string, dbFS embed.FS, log *slog.Logger) error {
	// Check if the database file exists
	entries, err := dbFS.ReadDir(".")
	if err != nil || len(entries) == 0 {
		//nolint:nilerr // If no migrations are found, we assume the database is not initialized.
		return nil
	}

	// Validate the DSN
	dburl, err := url.Parse(dsn)
	if err != nil {
		return err
	}

	// Create dbmate instance
	db := dbmate.New(dburl)
	_, err = db.Driver()
	if err != nil {
		return err
	}
	db.FS = dbFS
	db.AutoDumpSchema = false

	// List migrations
	migrations, err := db.FindMigrations()
	if err != nil {
		return err
	}
	for _, m := range migrations {
		log.Info("Migration", "version", m.Version, "file", m.FilePath)
	}

	// Apply migrations
	log.Info("Applying...")
	err = db.CreateAndMigrate()
	if err != nil {
		return err
	}

	return nil
}
