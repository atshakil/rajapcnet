package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"nvr/internal/model"
)

func (h *handler) listCameras(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`
		SELECT id, name, ip, port, rtsp_port, username, onvif_path, stream_path, enabled, created_at, updated_at
		FROM cameras ORDER BY id`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	cameras := []model.Camera{}
	for rows.Next() {
		var c model.Camera
		if err := rows.Scan(&c.ID, &c.Name, &c.IP, &c.Port, &c.RTSPPort, &c.Username,
			&c.ONVIFPath, &c.StreamPath, &c.Enabled, &c.CreatedAt, &c.UpdatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		cameras = append(cameras, c)
	}

	writeJSON(w, http.StatusOK, cameras)
}

func (h *handler) addCamera(w http.ResponseWriter, r *http.Request) {
	var c model.Camera
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if c.Name == "" || c.IP == "" {
		http.Error(w, "name and ip are required", http.StatusBadRequest)
		return
	}
	if c.Port == 0 {
		c.Port = 80
	}
	if c.RTSPPort == 0 {
		c.RTSPPort = 554
	}
	if c.ONVIFPath == "" {
		c.ONVIFPath = "/onvif/device_service"
	}

	result, err := h.db.Exec(
		`INSERT INTO cameras (name, ip, port, rtsp_port, username, password, onvif_path, stream_path, enabled)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Name, c.IP, c.Port, c.RTSPPort, c.Username, c.Password, c.ONVIFPath, c.StreamPath, true,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	c.ID, _ = result.LastInsertId()
	c.Enabled = true
	c.Password = ""
	writeJSON(w, http.StatusCreated, c)
}

func (h *handler) getCamera(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var c model.Camera
	err = h.db.QueryRow(
		`SELECT id, name, ip, port, rtsp_port, username, onvif_path, stream_path, enabled, created_at, updated_at
		 FROM cameras WHERE id = ?`, id,
	).Scan(&c.ID, &c.Name, &c.IP, &c.Port, &c.RTSPPort, &c.Username,
		&c.ONVIFPath, &c.StreamPath, &c.Enabled, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, c)
}

func (h *handler) updateCamera(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var c model.Camera
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	_, err = h.db.Exec(
		`UPDATE cameras SET name=?, ip=?, port=?, rtsp_port=?, username=?, password=?, onvif_path=?, stream_path=?, enabled=?, updated_at=CURRENT_TIMESTAMP
		 WHERE id=?`,
		c.Name, c.IP, c.Port, c.RTSPPort, c.Username, c.Password, c.ONVIFPath, c.StreamPath, c.Enabled, id,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) deleteCamera(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	_, err = h.db.Exec(`DELETE FROM cameras WHERE id = ?`, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
