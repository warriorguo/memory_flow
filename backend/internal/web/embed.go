// Package web embeds the built frontend (frontend/dist, copied here by the
// build step) and serves it as a single-page app for the standalone binary.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// Handler serves the embedded SPA: static files when they exist, otherwise
// index.html (so client-side routes resolve). Returns ok=false if the build
// step did not populate dist (no index.html), letting the caller decide.
func Handler() (http.Handler, bool) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, false
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil, false
	}

	fileServer := http.FileServer(http.FS(sub))
	index, _ := fs.ReadFile(sub, "index.html")

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" {
			serveIndex(w, index)
			return
		}
		if f, err := sub.Open(p); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// Unknown path with no file extension -> SPA route, serve index.
		serveIndex(w, index)
	})
	return h, true
}

func serveIndex(w http.ResponseWriter, index []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(index)
}
