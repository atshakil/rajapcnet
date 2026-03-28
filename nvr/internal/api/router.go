package api

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"nvr/internal/config"
	"nvr/internal/go2rtc"
	"nvr/internal/motion"
)

type handler struct {
	db        *sql.DB
	cfg       *config.Config
	jwtSecret []byte
	go2rtc    *go2rtc.Client
	motion    *motion.Manager
}

func NewServer(db *sql.DB, cfg *config.Config, motionMgr *motion.Manager) *http.Server {
	h := &handler{
		db:        db,
		cfg:       cfg,
		jwtSecret: []byte(cfg.JWTSecret),
		go2rtc:    go2rtc.NewClient(cfg.Go2RTCAddr),
		motion:    motionMgr,
	}

	// Pre-populate go2rtc with existing camera streams in the background.
	// This handles the case where go2rtc restarted while NVR was running.
	if h.go2rtc.Enabled() {
		go h.syncGo2RTCStreams()
	}

	mux := http.NewServeMux()

	// Public
	mux.HandleFunc("GET /api/health", h.health)
	mux.HandleFunc("POST /api/login", h.login)
	mux.HandleFunc("POST /api/bootstrap", h.bootstrap)

	// Protected API — requires valid JWT
	api := http.NewServeMux()
	api.HandleFunc("GET /api/cameras", h.listCameras)
	api.HandleFunc("POST /api/cameras", h.addCamera)
	api.HandleFunc("GET /api/cameras/{id}", h.getCamera)
	api.HandleFunc("GET /api/cameras/{id}/snapshot", h.cameraSnapshot)
	api.HandleFunc("POST /api/cameras/{id}/webrtc", h.cameraWebRTC)
	api.HandleFunc("POST /api/cameras/{id}/set-h264", h.setCameraCodec)
	api.HandleFunc("PUT /api/cameras/{id}/pref", h.setStreamPref)
	api.HandleFunc("PUT /api/cameras/{id}", h.updateCamera)
	api.HandleFunc("DELETE /api/cameras/{id}", h.deleteCamera)
	api.HandleFunc("GET /api/prefs", h.listPrefs)
	api.HandleFunc("GET /api/users", h.adminOnly(h.listUsers))
	api.HandleFunc("POST /api/users", h.adminOnly(h.addUser))
	api.HandleFunc("GET /api/users/{id}", h.adminOnly(h.getUser))
	api.HandleFunc("PUT /api/users/{id}", h.adminOnly(h.updateUser))
	api.HandleFunc("DELETE /api/users/{id}", h.adminOnly(h.deleteUser))

	// Motion logging
	api.HandleFunc("GET /api/cameras/{id}/motion-log", h.getMotionSettings)
	api.HandleFunc("PUT /api/cameras/{id}/motion-log", h.adminOnly(h.updateMotionSettings))
	api.HandleFunc("GET /api/cameras/{id}/motion-log/events", h.listCameraMotionEvents)
	api.HandleFunc("GET /api/cameras/{id}/motion-log/stream", h.streamCameraMotionEvents)
	api.HandleFunc("GET /api/motion-log/events", h.listMotionEvents)
	api.HandleFunc("GET /api/motion-log/stream", h.streamMotionEvents)
	api.HandleFunc("GET /api/motion-log/status", h.motionLogStatus)

	mux.Handle("/api/", h.authMiddleware(api))

	// Static web UI (less specific path than /api/, so API routes win)
	mux.HandleFunc("/", h.serveWeb)

	return &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: mux,
	}
}

func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// syncGo2RTCStreams registers all enabled camera streams in go2rtc on startup.
// Runs in a goroutine; waits briefly for go2rtc to finish starting.
func (h *handler) syncGo2RTCStreams() {
	time.Sleep(3 * time.Second)
	rows, err := h.db.Query(`SELECT id, username, password, stream_uris FROM cameras WHERE enabled = 1`)
	if err != nil {
		log.Printf("go2rtc sync: query cameras: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var username, password, streamURIs string
		if err := rows.Scan(&id, &username, &password, &streamURIs); err != nil {
			continue
		}
		var uris []string
		json.Unmarshal([]byte(streamURIs), &uris) //nolint:errcheck
		h.go2rtc.RegisterCameraStreams(id, username, password, uris)
	}
	log.Printf("go2rtc: startup stream sync complete")
}
