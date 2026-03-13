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
}
