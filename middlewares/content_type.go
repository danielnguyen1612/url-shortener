package middlewares

import (
	"net/http"
	"strings"
)

// AllowContentType enforces a whitelist of request Content-Types otherwise responds
// with a 415 Unsupported Media Type status.
func AllowContentType(contentTypes ...string) func(next http.Handler) http.Handler {
	cT := []string{}
	for _, t := range contentTypes {
		cT = append(cT, strings.ToLower(t))
	}

	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			s := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
			if len(s) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			if i := strings.Index(s, ";"); i > -1 {
				s = s[0:i]
			}

			for _, t := range cT {
				if t == s {
					next.ServeHTTP(w, r)
					return
				}
			}

			w.WriteHeader(http.StatusUnsupportedMediaType)
		}
		return http.HandlerFunc(fn)
	}
}
