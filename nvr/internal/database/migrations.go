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
