package model

import "time"

type Camera struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	IP         string    `json:"ip"`
	Port       int       `json:"port"`
	RTSPPort   int       `json:"rtsp_port"`
	Username   string    `json:"username,omitempty"`
	Password   string    `json:"password,omitempty"`
	ONVIFPath  string    `json:"onvif_path"`
	StreamPath string    `json:"stream_path"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	// Capabilities discovered via ONVIF
	Manufacturer  string `json:"manufacturer,omitempty"`
	Model         string `json:"model,omitempty"`
	Firmware      string `json:"firmware,omitempty"`
	HasONVIF      bool   `json:"has_onvif"`
	HasPTZ        bool   `json:"has_ptz"`
	HasMotion     bool   `json:"has_motion"`
	HasInfrared   bool   `json:"has_infrared"`
	HasFloodlight bool   `json:"has_floodlight"`
	HasIndicator  bool   `json:"has_indicator"`
	Resolutions   string `json:"resolutions,omitempty"`   // JSON array stored as text
	StreamURIs    string `json:"stream_uris,omitempty"`   // JSON array stored as text
	SnapshotURIs  string `json:"snapshot_uris,omitempty"` // JSON array stored as text
}
