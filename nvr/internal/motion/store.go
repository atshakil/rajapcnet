package motion

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nvr/internal/model"
)

// Store handles SQLite persistence for motion settings and episodes.
type Store struct {
	db *sql.DB
}

// NewStore creates a Store backed by the given database connection.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// ---------------------------------------------------------------------------
// Settings

// GetSettings returns motion settings for a camera, or defaults if none exist.
func (s *Store) GetSettings(cameraID int64) (*model.MotionSettings, error) {
	var ms model.MotionSettings
	err := s.db.QueryRow(
		`SELECT camera_id, enabled, retention_days, updated_at_ms FROM camera_motion_settings WHERE camera_id = ?`,
		cameraID,
	).Scan(&ms.CameraID, &ms.Enabled, &ms.RetentionDays, &ms.UpdatedAtMs)
	if err == sql.ErrNoRows {
		return &model.MotionSettings{CameraID: cameraID, RetentionDays: 7}, nil
	}
	if err != nil {
		return nil, err
	}
	return &ms, nil
}

// UpsertSettings creates or updates motion settings for a camera.
func (s *Store) UpsertSettings(ms *model.MotionSettings) error {
	ms.UpdatedAtMs = time.Now().UnixMilli()
	_, err := s.db.Exec(
		`INSERT INTO camera_motion_settings (camera_id, enabled, retention_days, updated_at_ms)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(camera_id) DO UPDATE SET enabled=excluded.enabled, retention_days=excluded.retention_days, updated_at_ms=excluded.updated_at_ms`,
		ms.CameraID, ms.Enabled, ms.RetentionDays, ms.UpdatedAtMs,
	)
	return err
}

// ListEnabled returns all camera IDs with motion logging enabled.
func (s *Store) ListEnabled() ([]model.MotionSettings, error) {
	rows, err := s.db.Query(`SELECT camera_id, enabled, retention_days, updated_at_ms FROM camera_motion_settings WHERE enabled = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []model.MotionSettings
	for rows.Next() {
		var ms model.MotionSettings
		if err := rows.Scan(&ms.CameraID, &ms.Enabled, &ms.RetentionDays, &ms.UpdatedAtMs); err != nil {
			return nil, err
		}
		result = append(result, ms)
	}
	return result, nil
}

// AllSettings returns motion settings for all cameras that have a row.
func (s *Store) AllSettings() ([]model.MotionSettings, error) {
	rows, err := s.db.Query(`SELECT camera_id, enabled, retention_days, updated_at_ms FROM camera_motion_settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []model.MotionSettings
	for rows.Next() {
		var ms model.MotionSettings
		if err := rows.Scan(&ms.CameraID, &ms.Enabled, &ms.RetentionDays, &ms.UpdatedAtMs); err != nil {
			return nil, err
		}
		result = append(result, ms)
	}
	return result, nil
}

// DeleteSettings removes motion settings for a camera.
func (s *Store) DeleteSettings(cameraID int64) error {
	_, err := s.db.Exec(`DELETE FROM camera_motion_settings WHERE camera_id = ?`, cameraID)
	return err
}

// ---------------------------------------------------------------------------
// Episodes

// OpenEpisode returns the current open episode for a camera, or nil.
func (s *Store) OpenEpisode(cameraID int64) (*model.MotionEpisode, error) {
	var ep model.MotionEpisode
	err := s.db.QueryRow(
		`SELECT id, camera_id, source, started_at_ms, ended_at_ms, duration_ms, status, close_reason,
		        last_seen_at_ms, event_count, topic, rule_name, source_token, object_token,
		        first_camera_ts_ms, last_camera_ts_ms, meta_json
		 FROM motion_episodes WHERE camera_id = ? AND status = 'open' ORDER BY id DESC LIMIT 1`,
		cameraID,
	).Scan(&ep.ID, &ep.CameraID, &ep.Source, &ep.StartedAtMs, &ep.EndedAtMs, &ep.DurationMs,
		&ep.Status, &ep.CloseReason, &ep.LastSeenAtMs, &ep.EventCount,
		&ep.Topic, &ep.RuleName, &ep.SourceToken, &ep.ObjectToken,
		&ep.FirstCameraMs, &ep.LastCameraMs, &ep.MetaJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ep, nil
}

// InsertEpisode creates a new open motion episode and returns it with the inserted ID.
func (s *Store) InsertEpisode(ep *model.MotionEpisode) error {
	result, err := s.db.Exec(
		`INSERT INTO motion_episodes (camera_id, source, started_at_ms, status, last_seen_at_ms, event_count,
		  topic, rule_name, source_token, object_token, first_camera_ts_ms)
		 VALUES (?, ?, ?, 'open', ?, 1, ?, ?, ?, ?, ?)`,
		ep.CameraID, ep.Source, ep.StartedAtMs, ep.LastSeenAtMs,
		ep.Topic, ep.RuleName, ep.SourceToken, ep.ObjectToken, ep.FirstCameraMs,
	)
	if err != nil {
		return err
	}
	ep.ID, _ = result.LastInsertId()
	ep.Status = "open"
	ep.EventCount = 1
	return nil
}

// BumpEpisode updates last_seen and increments event_count on an open episode.
func (s *Store) BumpEpisode(id int64, nowMs int64, cameraMs *int64) error {
	_, err := s.db.Exec(
		`UPDATE motion_episodes SET last_seen_at_ms = ?, event_count = event_count + 1, last_camera_ts_ms = ? WHERE id = ?`,
		nowMs, cameraMs, id,
	)
	return err
}

// CloseEpisode finalises an open episode.
func (s *Store) CloseEpisode(id int64, endedMs int64, status, reason string) error {
	_, err := s.db.Exec(
		`UPDATE motion_episodes SET ended_at_ms = ?, duration_ms = ? - started_at_ms, status = ?, close_reason = ?, last_seen_at_ms = ? WHERE id = ?`,
		endedMs, endedMs, status, reason, endedMs, id,
	)
	return err
}

// CloseStaleEpisodes marks any open episodes as interrupted (used on startup).
func (s *Store) CloseStaleEpisodes(nowMs int64) (int64, error) {
	result, err := s.db.Exec(
		`UPDATE motion_episodes SET ended_at_ms = ?, duration_ms = ? - started_at_ms, status = 'interrupted', close_reason = 'process_restart'
		 WHERE status = 'open'`,
		nowMs, nowMs,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ListEpisodes returns a page of episodes matching the filter criteria.
// Uses keyset pagination with cursor = "started_at_ms,id".
func (s *Store) ListEpisodes(cameraID int64, fromMs, toMs int64, status string, cursor string, limit int) (*model.EpisodePage, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var cursorMs int64
	var cursorID int64
	if cursor != "" {
		parts := strings.SplitN(cursor, ",", 2)
		if len(parts) == 2 {
			cursorMs, _ = strconv.ParseInt(parts[0], 10, 64)
			cursorID, _ = strconv.ParseInt(parts[1], 10, 64)
		}
	}

	where := []string{"1=1"}
	args := []any{}

	if cameraID > 0 {
		where = append(where, "camera_id = ?")
		args = append(args, cameraID)
	}
	if fromMs > 0 {
		where = append(where, "started_at_ms >= ?")
		args = append(args, fromMs)
	}
	if toMs > 0 {
		where = append(where, "started_at_ms <= ?")
		args = append(args, toMs)
	}
	if status != "" {
		where = append(where, "status = ?")
		args = append(args, status)
	}
	if cursorMs > 0 || cursorID > 0 {
		where = append(where, "(started_at_ms > ? OR (started_at_ms = ? AND id > ?))")
		args = append(args, cursorMs, cursorMs, cursorID)
	}

	query := fmt.Sprintf(
		`SELECT id, camera_id, source, started_at_ms, ended_at_ms, duration_ms, status, close_reason,
		        last_seen_at_ms, event_count, topic, rule_name, source_token, object_token,
		        first_camera_ts_ms, last_camera_ts_ms, meta_json
		 FROM motion_episodes WHERE %s ORDER BY started_at_ms ASC, id ASC LIMIT ?`,
		strings.Join(where, " AND "),
	)
	args = append(args, limit+1) // fetch one extra to detect next page

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var episodes []model.MotionEpisode
	for rows.Next() {
		var ep model.MotionEpisode
		if err := rows.Scan(&ep.ID, &ep.CameraID, &ep.Source, &ep.StartedAtMs, &ep.EndedAtMs, &ep.DurationMs,
			&ep.Status, &ep.CloseReason, &ep.LastSeenAtMs, &ep.EventCount,
			&ep.Topic, &ep.RuleName, &ep.SourceToken, &ep.ObjectToken,
			&ep.FirstCameraMs, &ep.LastCameraMs, &ep.MetaJSON); err != nil {
			return nil, err
		}
		episodes = append(episodes, ep)
	}

	page := &model.EpisodePage{}
	if len(episodes) > limit {
		episodes = episodes[:limit]
		last := episodes[limit-1]
		page.NextCursor = fmt.Sprintf("%d,%d", last.StartedAtMs, last.ID)
	}
	if episodes == nil {
		episodes = []model.MotionEpisode{}
	}
	page.Episodes = episodes
	return page, nil
}

// DeleteExpired removes closed episodes past their retention in batches.
func (s *Store) DeleteExpired(nowMs int64, batchSize int) (int64, error) {
	result, err := s.db.Exec(
		`DELETE FROM motion_episodes WHERE id IN (
			SELECT e.id FROM motion_episodes e
			JOIN camera_motion_settings s ON e.camera_id = s.camera_id
			WHERE e.status != 'open'
			AND s.retention_days > 0
			AND e.started_at_ms < (? - s.retention_days * 86400000)
			LIMIT ?
		)`,
		nowMs, batchSize,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// DeleteByCameraID removes all episodes for a camera.
func (s *Store) DeleteByCameraID(cameraID int64) error {
	_, err := s.db.Exec(`DELETE FROM motion_episodes WHERE camera_id = ?`, cameraID)
	return err
}
