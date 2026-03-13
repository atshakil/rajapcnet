package database

import "database/sql"

func Migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS cameras (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			name        TEXT    NOT NULL,
			ip          TEXT    NOT NULL,
			port        INTEGER NOT NULL DEFAULT 80,
			rtsp_port   INTEGER NOT NULL DEFAULT 554,
			username    TEXT    NOT NULL DEFAULT '',
			password    TEXT    NOT NULL DEFAULT '',
			onvif_path  TEXT    NOT NULL DEFAULT '/onvif/device_service',
			stream_path TEXT    NOT NULL DEFAULT '',
			enabled     INTEGER NOT NULL DEFAULT 1,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}
