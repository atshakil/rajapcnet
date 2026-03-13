package api

import (
	"database/sql"
	"net/http"

	"nvr/internal/config"
)

type handler struct {
	db  *sql.DB
	cfg *config.Config
}

func NewServer(db *sql.DB, cfg *config.Config) *http.Server {
	mux := http.NewServeMux()
	h := &handler{db: db, cfg: cfg}

	mux.HandleFunc("GET /api/health", h.health)
	mux.HandleFunc("GET /api/cameras", h.listCameras)
	mux.HandleFunc("POST /api/cameras", h.addCamera)
	mux.HandleFunc("GET /api/cameras/{id}", h.getCamera)
	mux.HandleFunc("PUT /api/cameras/{id}", h.updateCamera)
	mux.HandleFunc("DELETE /api/cameras/{id}", h.deleteCamera)

	return &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: mux,
	}
}

func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}
