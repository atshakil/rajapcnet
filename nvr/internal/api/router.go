package api

import (
	"database/sql"
	"net/http"

	"nvr/internal/config"
)

type handler struct {
	db        *sql.DB
	cfg       *config.Config
	jwtSecret []byte
}

func NewServer(db *sql.DB, cfg *config.Config) *http.Server {
	h := &handler{
		db:        db,
		cfg:       cfg,
		jwtSecret: []byte(cfg.JWTSecret),
	}

	mux := http.NewServeMux()

	// Public
	mux.HandleFunc("GET /api/health", h.health)
	mux.HandleFunc("POST /api/login", h.login)

	// Protected API — requires valid JWT
	api := http.NewServeMux()
	api.HandleFunc("GET /api/cameras", h.listCameras)
	api.HandleFunc("POST /api/cameras", h.addCamera)
	api.HandleFunc("GET /api/cameras/{id}", h.getCamera)
	api.HandleFunc("PUT /api/cameras/{id}", h.updateCamera)
	api.HandleFunc("DELETE /api/cameras/{id}", h.deleteCamera)
	api.HandleFunc("GET /api/users", h.adminOnly(h.listUsers))
	api.HandleFunc("POST /api/users", h.adminOnly(h.addUser))
	api.HandleFunc("GET /api/users/{id}", h.adminOnly(h.getUser))
	api.HandleFunc("PUT /api/users/{id}", h.adminOnly(h.updateUser))
	api.HandleFunc("DELETE /api/users/{id}", h.adminOnly(h.deleteUser))

	mux.Handle("/api/", h.authMiddleware(api))

	// Static web UI
	mux.HandleFunc("GET /", h.serveWeb)

	return &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: mux,
	}
}

func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}
