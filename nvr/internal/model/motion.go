package model

// MotionSettings holds per-camera motion logging configuration.
type MotionSettings struct {
	CameraID      int64  `json:"camera_id"`
	Enabled       bool   `json:"enabled"`
	RetentionDays int    `json:"retention_days"`
	UpdatedAtMs   int64  `json:"updated_at_ms"`
	RuntimeState  string `json:"runtime_state,omitempty"` // populated at runtime, not stored
}

// MotionEpisode represents a normalized motion event episode.
type MotionEpisode struct {
	ID            int64   `json:"id"`
	CameraID      int64   `json:"camera_id"`
	Source        string  `json:"source"`
	StartedAtMs   int64   `json:"started_at_ms"`
	EndedAtMs     *int64  `json:"ended_at_ms"`
	DurationMs    *int64  `json:"duration_ms"`
	Status        string  `json:"status"` // open, closed, inferred_closed, interrupted
	CloseReason   string  `json:"close_reason,omitempty"`
	LastSeenAtMs  int64   `json:"last_seen_at_ms"`
	EventCount    int     `json:"event_count"`
	Topic         string  `json:"topic,omitempty"`
	RuleName      string  `json:"rule_name,omitempty"`
	SourceToken   string  `json:"source_token,omitempty"`
	ObjectToken   string  `json:"object_token,omitempty"`
	FirstCameraMs *int64  `json:"first_camera_ts_ms"`
	LastCameraMs  *int64  `json:"last_camera_ts_ms"`
	MetaJSON      *string `json:"meta_json,omitempty"`
}

// MotionEvent is a compact SSE event published to live consumers.
type MotionEvent struct {
	Type       string `json:"type"`                  // motion.start, motion.end, motion.update, motion.runtime, motion.error
	CameraID   int64  `json:"camera_id"`
	CameraName string `json:"camera_name,omitempty"`
	EpisodeID  int64  `json:"episode_id,omitempty"`
	TimestampMs int64 `json:"timestamp_ms"`
	DurationMs *int64 `json:"duration_ms,omitempty"`
	EventCount int    `json:"event_count,omitempty"`
	State      string `json:"state,omitempty"` // for runtime events
	Error      string `json:"error,omitempty"` // for error events
}

// EpisodePage is a paginated result of motion episodes.
type EpisodePage struct {
	Episodes   []MotionEpisode `json:"episodes"`
	NextCursor string          `json:"next_cursor,omitempty"`
}
