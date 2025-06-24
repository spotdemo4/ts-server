package client

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/spotdemo4/ts-server/internal/auth"
	"github.com/spotdemo4/ts-server/internal/interceptors"
)

func NewClientHandler(auth *auth.Auth, clientFS embed.FS) http.Handler {
	entries, err := clientFS.ReadDir(".")
	if err != nil || len(entries) == 0 {
		return http.NotFoundHandler()
	}

	client, err := fs.Sub(clientFS, "client")
	if err != nil {
		return http.NotFoundHandler()
	}

	fs := http.FS(client)
	return interceptors.WithAuthRedirect(http.FileServer(fs), auth)
}
