package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"
)

func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger := getLoggerFromContext(r.Context())
				logger.Error("panic recovered",
					"error", err,
					"stack", string(debug.Stack()),
					"path", r.URL.Path,
				)

				w.WriteHeader(http.StatusInternalServerError)
				_, _ = fmt.Fprintf(w, "Internal Server Error")
			}
		}()

		next.ServeHTTP(w, r)
	})
}
