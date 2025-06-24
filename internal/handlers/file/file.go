package file

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/spotdemo4/ts-server/internal/auth"
	"github.com/spotdemo4/ts-server/internal/interceptors"
	"github.com/spotdemo4/ts-server/internal/models"
	"github.com/stephenafamo/bob"
)

type FileHandler struct {
	db   *bob.DB
	auth *auth.Auth
}

func (h *FileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	if len(pathItems) < 3 {
		http.Redirect(w, r, "/auth", http.StatusFound)
		return
	}
	id, err := strconv.Atoi(pathItems[2])
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

	w.Header().Set("Content-Type", http.DetectContentType(file.Data))
	w.Write(file.Data)
}

func NewFileHandler(db *bob.DB, auth *auth.Auth) http.Handler {
	return interceptors.WithAuthRedirect(
		&FileHandler{
			db:   db,
			auth: auth,
		},
		auth,
	)
}
