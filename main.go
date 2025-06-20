// TrevStack HTTP Server
package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/validate"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/joho/godotenv"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/spotdemo4/ts-server/internal/database"
	"github.com/spotdemo4/ts-server/internal/handlers/client"
	"github.com/spotdemo4/ts-server/internal/handlers/file"
	"github.com/spotdemo4/ts-server/internal/handlers/item/v1"
	"github.com/spotdemo4/ts-server/internal/handlers/user/v1"
	"github.com/spotdemo4/ts-server/internal/interceptors"
)

var ClientFS embed.FS
var DBFS embed.FS

func main() {
	name := "TrevStack"

	// Get env
	env, err := getEnv()
	if err != nil {
		log.Fatal(err.Error())
	}

	// Migrate database
	err = database.Migrate(env.DatabaseURL, DBFS)
	if err != nil {
		log.Fatal(err.Error())
	}

	// Get database
	sqlc, db, err := database.New(env.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %s", err.Error())
	}

	// Create webauthn
	webAuthn, err := webauthn.New(&webauthn.Config{
		RPDisplayName: name,
		RPID:          env.URL.Hostname(),
		RPOrigins:     []string{env.URL.String()},
	})
	if err != nil {
		log.Fatalf("failed to create webauthn: %s", err.Error())
	}

	// Create validate interceptor
	vi, err := validate.NewInterceptor()
	if err != nil {
		log.Fatalf("failed to create validator: %s", err.Error())
	}

	// Serve gRPC Handlers
	api := http.NewServeMux()
	api.Handle(interceptors.WithCORS(user.NewAuthHandler(vi, sqlc, webAuthn, name, env.Key)))
	api.Handle(interceptors.WithCORS(user.NewHandler(vi, sqlc, webAuthn, name, env.Key)))
	api.Handle(interceptors.WithCORS(item.NewHandler(vi, sqlc, env.Key)))

	// Serve web interface
	mux := http.NewServeMux()
	mux.Handle("/", client.NewClientHandler(env.Key, ClientFS))
	mux.Handle("/file/", file.NewFileHandler(sqlc, env.Key))
	mux.Handle("/grpc/", http.StripPrefix("/grpc", api))

	// Start server
	log.Printf("Starting server on :%s", env.Port)
	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", env.Port),
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}

	// Gracefully shutdown on SIGINT or SIGTERM
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		log.Printf("Received signal %s, exiting", sig)

		// Close HTTP server
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := server.Shutdown(ctx); err != nil {
			server.Close()
		}
		cancel()

		// Close database connection
		db.Close()
	}()

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

type env struct {
	Port        string
	Key         string
	URL         *url.URL
	DatabaseURL string
}

func getEnv() (*env, error) {
	err := godotenv.Load()
	if err != nil {
		log.Println("Failed to load .env file, using environment variables")
	}

	// Create
	env := env{
		Port:        os.Getenv("PORT"),
		Key:         os.Getenv("KEY"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}

	// Validate
	if env.Port == "" {
		env.Port = "8080"
		log.Printf("env 'PORT' not found, defaulting to %s\n", env.Port)
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
		log.Printf("env 'URL' not found, defaulting to %s\n", env.URL.String())
	} else {
		env.URL, err = url.Parse(os.Getenv("URL"))
		if err != nil {
			return nil, err
		}
	}

	return &env, nil
}
