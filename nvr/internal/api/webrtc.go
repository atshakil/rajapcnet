package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"nvr/internal/model"
)

// ---------------------------------------------------------------------------
// Stream preference handlers

// listPrefs returns all per-camera stream_mode preferences for the
// authenticated user as: {"cameras": {"14": "primary", ...}}
func (h *handler) listPrefs(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userContextKey).(*model.User)

	rows, err := h.db.Query(
		`SELECT camera_id, stream_mode FROM user_camera_prefs WHERE user_id = ?`, u.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	prefs := map[string]string{}
	for rows.Next() {
		var camID int64
		var mode string
		if err := rows.Scan(&camID, &mode); err == nil {
			prefs[strconv.FormatInt(camID, 10)] = mode
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"cameras": prefs})
}

// setStreamPref upserts the stream_mode for one camera for the current user.
// Body: {"stream_mode": "snapshot"|"primary"|"sub"}
func (h *handler) setStreamPref(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userContextKey).(*model.User)

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var req struct {
		StreamMode string `json:"stream_mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	validModes := map[string]bool{"snapshot": true, "primary": true, "sub": true}
	if !validModes[req.StreamMode] {
		http.Error(w, "stream_mode must be snapshot, primary, or sub", http.StatusBadRequest)
		return
	}

	_, err = h.db.Exec(
		`INSERT INTO user_camera_prefs (user_id, camera_id, stream_mode) VALUES (?, ?, ?)
		 ON CONFLICT(user_id, camera_id) DO UPDATE SET stream_mode = excluded.stream_mode`,
		u.ID, id, req.StreamMode,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// WebRTC proxy handler

// cameraWebRTC proxies a WebRTC SDP offer/answer exchange through go2rtc.
//
// Query param: stream=primary (default) | sub
// Request body: raw SDP offer (Content-Type: application/sdp)
// Response: raw SDP answer
//
// go2rtc is registered lazily — we PUT the RTSP source before each offer so
// the stream is always fresh even if go2rtc restarted.
func (h *handler) cameraWebRTC(w http.ResponseWriter, r *http.Request) {
	if !h.go2rtc.Enabled() {
		http.Error(w, "WebRTC relay not configured (NVR_GO2RTC_ADDR)", http.StatusServiceUnavailable)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	streamParam := r.URL.Query().Get("stream")
	if streamParam == "" {
		streamParam = "primary"
	}

	streamIdx := 0
	if streamParam == "sub" {
		streamIdx = 1
	}

	// Look up camera credentials and stream URIs
	var username, password, streamURIs string
	err = h.db.QueryRow(
		`SELECT username, password, stream_uris FROM cameras WHERE id = ? AND enabled = 1`, id,
	).Scan(&username, &password, &streamURIs)
	if err != nil {
		http.Error(w, "camera not found", http.StatusNotFound)
		return
	}

	var uris []string
	if err := json.Unmarshal([]byte(streamURIs), &uris); err != nil || streamIdx >= len(uris) {
		http.Error(w, "stream not available", http.StatusNotFound)
		return
	}

	// Read SDP offer from request body
	offerSDP, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read offer", http.StatusBadRequest)
		return
	}

	// Register stream in go2rtc and exchange SDP
	answerSDP, err := h.go2rtc.WebRTC(id, streamIdx, username, password, uris[streamIdx], string(offerSDP))
	if err != nil {
		// Distinguish codec mismatch from other relay errors — lets the UI show
		// a helpful "Fix: switch to H.264" prompt instead of silently falling back.
		if strings.Contains(err.Error(), "codecs not matched") {
			camCodec := "H265"
			for _, field := range strings.Fields(err.Error()) {
				if strings.HasPrefix(field, "video:") {
					camCodec = strings.TrimRight(strings.TrimPrefix(field, "video:"), ",;")
					break
				}
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{
				"error":        "codec_mismatch",
				"camera_codec": camCodec,
				"message":      "Camera streams " + camCodec + ". Most browsers only support H.264 for WebRTC. Click Fix to switch the camera encoder to H.264 via ONVIF.",
			})
			return
		}
		http.Error(w, "WebRTC relay error: "+err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/sdp")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(answerSDP))
}
