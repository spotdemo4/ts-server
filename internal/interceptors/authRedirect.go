package interceptors

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/spotdemo4/ts-server/internal/auth"
)

func WithAuthRedirect(next http.Handler, auth *auth.Auth) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathItems := strings.Split(r.URL.Path, "/")

		// Check if the user is authenticated
		authenticated := false
		cookies := getCookies(r.Header.Get("Cookie"))
		for _, cookie := range cookies {
			if cookie.Name == CookieTokenName {
				user, err := auth.GetUserFromToken(cookie.Value)
				if err == nil {
					r = r.WithContext(auth.NewContext(r.Context(), user))
					authenticated = true
				}

				break
			}
		}

		switch pathItems[1] {
		case "auth":
			if authenticated {
				http.Redirect(w, r, "/", http.StatusFound)
				return
			}
			next.ServeHTTP(w, r)

		case "_app", "favicon.png", "icon.png":
			next.ServeHTTP(w, r)

		default:
			if authenticated {
				next.ServeHTTP(w, r)
				return
			}

			// Redirect if not authenticated
			pathRedir := url.QueryEscape(r.URL.Path)
			// nosemgrep
			http.Redirect(w, r, fmt.Sprintf("/auth?redir=%s", pathRedir), http.StatusFound)
		}
	})
}
