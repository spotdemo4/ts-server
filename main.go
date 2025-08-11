// TrevStack HTTP Server
package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/spotdemo4/ts-server/internal/app"
	"github.com/spotdemo4/ts-server/internal/handlers/client"
	"github.com/spotdemo4/ts-server/internal/handlers/file"
	itemv1 "github.com/spotdemo4/ts-server/internal/handlers/item/v1"
	userv1 "github.com/spotdemo4/ts-server/internal/handlers/user/v1"
	"github.com/spotdemo4/ts-server/internal/interceptors"
)

const Timeout = 10 * time.Second

//nolint:gochecknoglobals // Embed the web client
var clientFS embed.FS

//nolint:gochecknoglobals // Embed the database .sql files for migrations
var dbFS embed.FS

func main() {
	name := "TrevStack"

	// Create base application (log, env, database, auth)
	base, err := app.New(name, dbFS)
	if err != nil {
		log.Fatalf("failed to create app: %s", err.Error())
	}

	// Create interceptors
	li := interceptors.NewLoggingInterceptor(base.Log) // Logging interceptor for request logging
	ai := interceptors.NewAuthInterceptor(base.Auth)   // Auth interceptor for user authentication
	ri := interceptors.NewRateLimitInterceptor()       // Rate limit interceptor for protecting endpoints
	vi, err := validate.NewInterceptor()               // Validator interceptor for validating requests
	if err != nil {
		base.Log.Error("failed to create validator interceptor", "error", err)
		return
	}

	// Serve gRPC Handlers
	api := http.NewServeMux()
	api.Handle(interceptors.WithCORS(userv1.New(base, connect.WithInterceptors(li, vi, ai))))     // User handler
	api.Handle(interceptors.WithCORS(userv1.NewAuth(base, connect.WithInterceptors(li, vi, ri)))) // User auth handler
	api.Handle(interceptors.WithCORS(itemv1.New(base, connect.WithInterceptors(li, vi, ai))))     // Item handler

	// Serve web interface
	mux := http.NewServeMux()
	mux.Handle("/", client.New(base, clientFS))          // Web client handler
	mux.Handle("/file/", file.New(base))                 // File handler for serving files
	mux.Handle("/grpc/", http.StripPrefix("/grpc", api)) // gRPC API handler

	// Start server
	base.Log.Info("Starting server", "port", base.Env.Port)
	server := &http.Server{
		Addr:              fmt.Sprintf(":%s", base.Env.Port),
		Handler:           h2c.NewHandler(mux, &http2.Server{}),
		ReadHeaderTimeout: Timeout,
	}

	// Gracefully shutdown on SIGINT or SIGTERM
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		base.Log.Info("Received signal, shutting down", "signal", sig)

		// Close HTTP server
		ctx, cancel := context.WithTimeout(context.Background(), Timeout)
		if err = server.Shutdown(ctx); err != nil {
			err = server.Close()
			if err != nil {
				base.Log.Error("Failed to close server", "error", err)
			}
		}
		cancel()

		// Close database connection
		err = base.DB.Close()
		if err != nil {
			base.Log.Error("Failed to close database", "error", err)
		}
	}()

	if err = server.ListenAndServe(); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			base.Log.Error("Failed to start server", "error", err)
		}
	}
}
