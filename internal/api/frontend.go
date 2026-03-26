package api

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dist/*
var frontendFS embed.FS

// FrontendHandler serves the React dashboard from the embedded filesystem.
// Falls back to index.html for client-side routing (SPA).
func FrontendHandler() http.Handler {
	subFS, err := fs.Sub(frontendFS, "dist")
	if err != nil {
		panic("frontend: failed to create sub filesystem: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(subFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve static files (JS, CSS, images)
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Check if file exists in embedded FS
		if f, err := subFS.Open(path); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// File not found — serve index.html for SPA routing
		// This handles /login, /logs, /users, etc.
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
