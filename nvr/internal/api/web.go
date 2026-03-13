package api

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web/*
var webFS embed.FS

func (h *handler) serveWeb(w http.ResponseWriter, r *http.Request) {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Try to serve the requested file; fall back to index.html for SPA routing
	path := r.URL.Path
	if path == "/" {
		path = "index.html"
	} else {
		path = path[1:] // strip leading /
	}

	if _, err := fs.Stat(sub, path); err != nil {
		// File not found — serve index.html for client-side routing
		path = "index.html"
	}

	http.ServeFileFS(w, r, sub, path)
}
