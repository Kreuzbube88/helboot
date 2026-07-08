// Package web embeds the compiled frontend (ADR-0009). During the Docker
// build the frontend's dist output is copied into this package's dist/
// directory before `go build`; the repository only contains a placeholder
// page so API-only development builds work without Node.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// Handler serves the embedded single-page application. Unknown paths
// fall back to index.html so client-side routing works on reload.
func Handler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err) // impossible: dist is embedded above
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(sub, p); err != nil {
			if !os.IsNotExist(err) {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			// SPA fallback: let the client router handle the path.
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}
