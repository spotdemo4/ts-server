package file

import (
	"bytes"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/stephenafamo/bob"

	"github.com/spotdemo4/ts-server/internal/app"
	"github.com/spotdemo4/ts-server/internal/auth"
	"github.com/spotdemo4/ts-server/internal/bob/models"
	"github.com/spotdemo4/ts-server/internal/interceptors"
)

type Handler struct {
	db   *bob.DB
	auth *auth.Auth
}

const FilePathIndex = 2

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/auth", http.StatusFound)
		return
	}

	// Make sure this is a GET request
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the file id from the path
	pathItems := strings.Split(r.URL.Path, "/")
	if len(pathItems) <= FilePathIndex {
		http.Redirect(w, r, "/auth", http.StatusFound)
		return
	}
	id, err := strconv.ParseInt(pathItems[FilePathIndex], 10, 32)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Get file from db
	file, err := models.Files.Query(
		models.SelectWhere.Files.ID.EQ(int32(id)),
		models.SelectWhere.Files.UserID.EQ(user.ID),
	).One(r.Context(), h.db)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Not Found", http.StatusNotFound)
		}

		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Send file in response
	buffer := bytes.NewReader(file.Data)
	http.ServeContent(w, r, file.Name, time.Time{}, buffer)
}

func New(app *app.App) http.Handler {
	return interceptors.WithAuthRedirect(
		&Handler{
			db:   app.DB,
			auth: app.Auth,
		},
		app.Auth,
	)
}
