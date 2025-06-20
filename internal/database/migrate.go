package database

import (
	"embed"
	"log"
	"net/url"

	"github.com/amacneil/dbmate/v2/pkg/dbmate"
	_ "github.com/spotdemo4/dbmate-sqlite-modernc/pkg/driver/sqlite" // Modernc sqlite
)

func Migrate(dsn string, dbFS embed.FS) error {
	entries, err := dbFS.ReadDir(".")
	if err != nil || len(entries) == 0 {
		return nil
	}

	dburl, err := url.Parse(dsn)
	if err != nil {
		return err
	}

	db := dbmate.New(dburl)
	db.Driver()
	db.FS = dbFS
	db.AutoDumpSchema = false

	log.Println("Migrations:")
	migrations, err := db.FindMigrations()
	if err != nil {
		return err
	}
	for _, m := range migrations {
		log.Println(m.Version, m.FilePath)
	}

	log.Println("Applying...")
	err = db.CreateAndMigrate()
	if err != nil {
		return err
	}

	return nil
}
