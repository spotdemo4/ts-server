package app

import (
	"errors"
	"log/slog"
	"net/url"
	"os"

	"github.com/joho/godotenv"
)

type Env struct {
	Port        string
	Key         string
	URL         *url.URL
	DatabaseURL string
}

func getEnv(log *slog.Logger) (*Env, error) {
	err := godotenv.Load()
	if err != nil {
		log.Info("Failed to load .env file, using environment variables")
	}

	// Create
	env := Env{
		Port:        os.Getenv("PORT"),
		Key:         os.Getenv("KEY"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}

	// Validate
	if env.Port == "" {
		env.Port = "8080"
		log.Info("env 'PORT' not found, setting default", "port", env.Port)
	}
	if env.Key == "" {
		return nil, errors.New("env 'KEY' not found")
	}
	if env.DatabaseURL == "" {
		return nil, errors.New("env 'DATABASE_URL' not found")
	}

	// Parse URL
	if os.Getenv("URL") == "" {
		env.URL, _ = url.Parse("http://localhost:" + env.Port)
		log.Info("env 'URL' not found, defaulting default", "url", env.URL.String())
	} else {
		env.URL, err = url.Parse(os.Getenv("URL"))
		if err != nil {
			return nil, err
		}
	}

	return &env, nil
}
