package app

import (
	"embed"
	"log/slog"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/stephenafamo/bob"

	"github.com/spotdemo4/ts-server/internal/auth"
	"github.com/spotdemo4/ts-server/internal/database"
)

type App struct {
	Log  *slog.Logger
	Env  *Env
	DB   *bob.DB
	Auth *auth.Auth
}

func New(name string, dbFS embed.FS) (*App, error) {
	// Create logger
	logger := slog.Default()

	// Get environment variables
	env, err := getEnv(logger)
	if err != nil {
		return nil, err
	}

	// Migrate database
	err = database.Migrate(env.DatabaseURL, dbFS, logger)
	if err != nil {
		return nil, err
	}

	// Get database
	db, err := database.New(env.DatabaseURL)
	if err != nil {
		return nil, err
	}

	// Create webauthn config
	web, err := webauthn.New(&webauthn.Config{
		RPDisplayName: name,
		RPID:          env.URL.Hostname(),
		RPOrigins:     []string{env.URL.String()},
	})
	if err != nil {
		return nil, err
	}

	// Create auth service
	auth := auth.New(db, name, env.Key, web)

	return &App{
		Log:  logger,
		Env:  env,
		DB:   db,
		Auth: auth,
	}, nil
}
