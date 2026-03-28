package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"nvr/internal/model"
)

// getMotionSettings returns motion logging config for one camera.
func (h *handler) getMotionSettings(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	ms, err := h.motion.Store().GetSettings(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ms.RuntimeState = h.motion.WorkerStatus(id)
	writeJSON(w, http.StatusOK, ms)
}

// updateMotionSettings creates or updates motion logging config for one camera.
func (h *handler) updateMotionSettings(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var req struct {
		Enabled       *bool `json:"enabled"`
		RetentionDays *int  `json:"retention_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	ms, err := h.motion.Store().GetSettings(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if req.Enabled != nil {
		ms.Enabled = *req.Enabled
	}
	if req.RetentionDays != nil {
		if *req.RetentionDays < 1 {
			http.Error(w, "retention_days must be >= 1", http.StatusBadRequest)
			return
		}
		ms.RetentionDays = *req.RetentionDays
	}
	ms.CameraID = id

	if err := h.motion.Store().UpsertSettings(ms); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.motion.SetEnabled(id, ms.Enabled)
	ms.RuntimeState = h.motion.WorkerStatus(id)
	writeJSON(w, http.StatusOK, ms)
}

// listCameraMotionEvents returns paginated motion episodes for one camera.
func (h *handler) listCameraMotionEvents(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	h.listMotionEventsInner(w, r, id)
}

// listMotionEvents returns paginated motion episodes globally.
func (h *handler) listMotionEvents(w http.ResponseWriter, r *http.Request) {
	camID := int64(0)
	if s := r.URL.Query().Get("camera_id"); s != "" {
		camID, _ = strconv.ParseInt(s, 10, 64)
	}
	h.listMotionEventsInner(w, r, camID)
}

func (h *handler) listMotionEventsInner(w http.ResponseWriter, r *http.Request, cameraID int64) {
	q := r.URL.Query()
	fromMs, _ := strconv.ParseInt(q.Get("from"), 10, 64)
	toMs, _ := strconv.ParseInt(q.Get("to"), 10, 64)
	status := q.Get("status")
	cursor := q.Get("cursor")
	limit, _ := strconv.Atoi(q.Get("limit"))

	page, err := h.motion.Store().ListEpisodes(cameraID, fromMs, toMs, status, cursor, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

// motionLogStatus returns motion settings + runtime state for all cameras.
func (h *handler) motionLogStatus(w http.ResponseWriter, r *http.Request) {
	all, err := h.motion.Store().AllSettings()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for i := range all {
		all[i].RuntimeState = h.motion.WorkerStatus(all[i].CameraID)
	}
	writeJSON(w, http.StatusOK, all)
}

// ---------------------------------------------------------------------------
// SSE streaming

// streamMotionEvents streams all motion events via SSE (global).
func (h *handler) streamMotionEvents(w http.ResponseWriter, r *http.Request) {
	h.streamMotionEventsInner(w, r, 0)
}

// streamCameraMotionEvents streams motion events for one camera via SSE.
func (h *handler) streamCameraMotionEvents(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	h.streamMotionEventsInner(w, r, id)
}

func (h *handler) streamMotionEventsInner(w http.ResponseWriter, r *http.Request, cameraID int64) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Send initial runtime state for enabled cameras
	all, _ := h.motion.Store().AllSettings()
	for _, s := range all {
		if !s.Enabled {
			continue
		}
		if cameraID > 0 && s.CameraID != cameraID {
			continue
		}
		evt := model.MotionEvent{
			Type:        "motion.runtime",
			CameraID:    s.CameraID,
			TimestampMs: time.Now().UnixMilli(),
			State:       h.motion.WorkerStatus(s.CameraID),
		}
		// Check for active episode
		if epID := h.motion.ActiveEpisodeID(s.CameraID); epID > 0 {
			evt.Type = "motion.start"
			evt.EpisodeID = epID
		}
		data, _ := json.Marshal(evt)
		w.Write([]byte("event: " + evt.Type + "\ndata: "))
		w.Write(data)
		w.Write([]byte("\n\n"))
	}
	flusher.Flush()

	ch := h.motion.Hub().Subscribe(cameraID)
	defer h.motion.Hub().Unsubscribe(cameraID, ch)

	keepalive := time.NewTicker(30 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case evt := <-ch:
			data, _ := json.Marshal(evt)
			w.Write([]byte("event: " + evt.Type + "\ndata: "))
			w.Write(data)
			w.Write([]byte("\n\n"))
			flusher.Flush()
		case <-keepalive.C:
			w.Write([]byte(": keepalive\n\n"))
			flusher.Flush()
		}
	}
}
