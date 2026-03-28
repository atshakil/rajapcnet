package database

import "database/sql"

func Migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS cameras (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			name          TEXT    NOT NULL,
			ip            TEXT    NOT NULL,
			port          INTEGER NOT NULL DEFAULT 80,
			rtsp_port     INTEGER NOT NULL DEFAULT 554,
			username      TEXT    NOT NULL DEFAULT '',
			password      TEXT    NOT NULL DEFAULT '',
			onvif_path    TEXT    NOT NULL DEFAULT '/onvif/device_service',
			stream_path   TEXT    NOT NULL DEFAULT '',
			enabled       INTEGER NOT NULL DEFAULT 1,
			manufacturer  TEXT    NOT NULL DEFAULT '',
			model         TEXT    NOT NULL DEFAULT '',
			firmware      TEXT    NOT NULL DEFAULT '',
			has_onvif     INTEGER NOT NULL DEFAULT 0,
			has_ptz       INTEGER NOT NULL DEFAULT 0,
			has_motion    INTEGER NOT NULL DEFAULT 0,
			has_infrared  INTEGER NOT NULL DEFAULT 0,
			has_floodlight INTEGER NOT NULL DEFAULT 0,
			has_indicator INTEGER NOT NULL DEFAULT 0,
			resolutions   TEXT    NOT NULL DEFAULT '[]',
			stream_uris   TEXT    NOT NULL DEFAULT '[]',
			snapshot_uris TEXT    NOT NULL DEFAULT '[]',
			created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			username   TEXT    NOT NULL UNIQUE,
			password   TEXT    NOT NULL,
			role       TEXT    NOT NULL DEFAULT 'viewer',
			enabled    INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// Per-user, per-camera stream mode preference.
		// stream_mode: "snapshot" | "primary" | "sub"
		`CREATE TABLE IF NOT EXISTS user_camera_prefs (
			user_id     INTEGER NOT NULL,
			camera_id   INTEGER NOT NULL,
			stream_mode TEXT    NOT NULL DEFAULT 'snapshot',
			PRIMARY KEY (user_id, camera_id)
		)`,
	}

	// Motion logging tables
	motionStmts := []string{
		`CREATE TABLE IF NOT EXISTS camera_motion_settings (
			camera_id      INTEGER PRIMARY KEY,
			enabled        INTEGER NOT NULL DEFAULT 0,
			retention_days INTEGER NOT NULL DEFAULT 7,
			updated_at_ms  INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS motion_episodes (
			id                INTEGER PRIMARY KEY AUTOINCREMENT,
			camera_id         INTEGER NOT NULL,
			source            TEXT    NOT NULL DEFAULT 'onvif',
			started_at_ms     INTEGER NOT NULL,
			ended_at_ms       INTEGER,
			duration_ms       INTEGER,
			status            TEXT    NOT NULL DEFAULT 'open',
			close_reason      TEXT    NOT NULL DEFAULT '',
			last_seen_at_ms   INTEGER NOT NULL,
			event_count       INTEGER NOT NULL DEFAULT 1,
			topic             TEXT    NOT NULL DEFAULT '',
			rule_name         TEXT    NOT NULL DEFAULT '',
			source_token      TEXT    NOT NULL DEFAULT '',
			object_token      TEXT    NOT NULL DEFAULT '',
			first_camera_ts_ms INTEGER,
			last_camera_ts_ms  INTEGER,
			meta_json         TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_motion_ep_cam_status ON motion_episodes(camera_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_motion_ep_cam_time ON motion_episodes(camera_id, started_at_ms, id)`,
		`CREATE INDEX IF NOT EXISTS idx_motion_ep_time ON motion_episodes(started_at_ms, id)`,
	}
	for _, s := range motionStmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}

	// Add columns to cameras if they don't exist (for upgrades)
	alters := []string{
		"ALTER TABLE cameras ADD COLUMN manufacturer TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE cameras ADD COLUMN model TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE cameras ADD COLUMN firmware TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE cameras ADD COLUMN has_onvif INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE cameras ADD COLUMN has_ptz INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE cameras ADD COLUMN has_motion INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE cameras ADD COLUMN has_infrared INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE cameras ADD COLUMN has_floodlight INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE cameras ADD COLUMN has_indicator INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE cameras ADD COLUMN resolutions TEXT NOT NULL DEFAULT '[]'",
		"ALTER TABLE cameras ADD COLUMN stream_uris TEXT NOT NULL DEFAULT '[]'",
		"ALTER TABLE cameras ADD COLUMN snapshot_uris TEXT NOT NULL DEFAULT '[]'",
	}

	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}

	for _, a := range alters {
		db.Exec(a) // ignore "duplicate column" errors for existing DBs
	}

	return nil
}
