package file

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/spotdemo4/ts-server/internal/interceptors"
	"github.com/spotdemo4/ts-server/internal/sqlc"
)

type FileHandler struct {
	db  *sqlc.Queries
	key []byte
}

func (h *FileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userid, ok := interceptors.GetUserContext(r.Context())
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

	// Get the file from the database
	file, err := h.db.GetFile(r.Context(), sqlc.GetFileParams{
		ID:     int64(id),
		UserID: userid,
	})
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

func NewFileHandler(db *sqlc.Queries, key string) http.Handler {
	return interceptors.WithAuthRedirect(
		&FileHandler{
			db:  db,
			key: []byte(key),
		},
		key,
	)
}
