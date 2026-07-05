package url

import (
	"context"
	"net/http"
	"path"
	"strings"
)

type contextKey string

const urlFormatKey contextKey = "url_format"

func URLFormat(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ext := path.Ext(r.URL.Path)
		if ext != "" {
			r.URL.Path = strings.TrimSuffix(r.URL.Path, ext)
			ext = strings.TrimPrefix(ext, ".")
			ctx := context.WithValue(r.Context(), urlFormatKey, ext)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

func GetURLFormat(r *http.Request) string {
	format, _ := r.Context().Value(urlFormatKey).(string)
	return format
}
