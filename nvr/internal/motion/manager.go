package motion

import (
	"context"
	"database/sql"
	"log"
	"strings"
	"sync"
	"time"

	"nvr/internal/model"
	"nvr/internal/onvif"
)

const (
	idleTimeoutMs = 10_000
	cleanupBatch  = 500
)

// WorkerState describes the runtime state of a per-camera event worker.
type WorkerState string

const (
	StateStarting     WorkerState = "starting"
	StateActive       WorkerState = "active"
	StateUnsupported  WorkerState = "unsupported"
	StateMisconfigured WorkerState = "misconfigured"
	StateDegraded     WorkerState = "degraded"
	StateDisconnected WorkerState = "disconnected"
	StateStopped      WorkerState = "stopped"
)

// Manager coordinates per-camera ONVIF event workers.
type Manager struct {
	db      *sql.DB
	store   *Store
	hub     *Hub
	mu      sync.Mutex
	workers map[int64]*worker
	ctx     context.Context
	cancel  context.CancelFunc
}

type worker struct {
	cameraID int64
	state    WorkerState
	stateMu  sync.RWMutex
	cancel   context.CancelFunc
	// in-memory open episode tracking
	episode *openEpisode
}

type openEpisode struct {
	id         int64
	lastSeenMs int64
}

// NewManager creates a Manager. Call Start() to begin processing.
func NewManager(db *sql.DB) *Manager {
	return &Manager{
		db:      db,
		store:   NewStore(db),
		hub:     NewHub(),
		workers: make(map[int64]*worker),
	}
}

// Hub returns the live event hub for SSE consumers.
func (m *Manager) Hub() *Hub { return m.hub }

// Store returns the underlying store for API queries.
func (m *Manager) Store() *Store { return m.store }

// Start begins processing. Closes stale episodes, starts workers for enabled
// cameras, and starts the retention cleanup loop.
func (m *Manager) Start(parentCtx context.Context) {
	m.ctx, m.cancel = context.WithCancel(parentCtx)

	// Close any episodes left open from a previous crash/restart
	n, err := m.store.CloseStaleEpisodes(time.Now().UnixMilli())
	if err != nil {
		log.Printf("motion: close stale episodes: %v", err)
	} else if n > 0 {
		log.Printf("motion: closed %d stale episodes as interrupted", n)
	}

	// Start workers for cameras with motion logging enabled
	settings, err := m.store.ListEnabled()
	if err != nil {
		log.Printf("motion: list enabled settings: %v", err)
	}
	m.mu.Lock()
	for _, s := range settings {
		m.startWorkerLocked(s.CameraID)
	}
	m.mu.Unlock()

	if len(settings) > 0 {
		log.Printf("motion: started %d event workers", len(settings))
	}

	// Periodic retention cleanup
	go m.cleanupLoop()
}

// Stop cancels all workers and waits for cleanup.
func (m *Manager) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

// SetEnabled starts or stops the event worker for a camera.
func (m *Manager) SetEnabled(cameraID int64, enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if enabled {
		if _, ok := m.workers[cameraID]; !ok {
			m.startWorkerLocked(cameraID)
		}
	} else {
		if w, ok := m.workers[cameraID]; ok {
			w.cancel()
			delete(m.workers, cameraID)
		}
	}
}

// StopCamera stops the worker and removes data for a deleted camera.
func (m *Manager) StopCamera(cameraID int64) {
	m.mu.Lock()
	if w, ok := m.workers[cameraID]; ok {
		w.cancel()
		delete(m.workers, cameraID)
	}
	m.mu.Unlock()
}

// WorkerStatus returns the runtime state for a given camera.
func (m *Manager) WorkerStatus(cameraID int64) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if w, ok := m.workers[cameraID]; ok {
		w.stateMu.RLock()
		defer w.stateMu.RUnlock()
		return string(w.state)
	}
	return string(StateStopped)
}

// ActiveEpisodeID returns the open episode ID for a camera, or 0.
func (m *Manager) ActiveEpisodeID(cameraID int64) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if w, ok := m.workers[cameraID]; ok {
		if w.episode != nil {
			return w.episode.id
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// worker lifecycle

func (m *Manager) startWorkerLocked(cameraID int64) {
	ctx, cancel := context.WithCancel(m.ctx)
	w := &worker{
		cameraID: cameraID,
		state:    StateStarting,
		cancel:   cancel,
	}
	m.workers[cameraID] = w
	go m.runWorker(ctx, w)
}

// runWorker is the main loop for a per-camera event worker.
// On error it reconnects with exponential backoff.
func (m *Manager) runWorker(ctx context.Context, w *worker) {
	backoff := 2 * time.Second
	maxBackoff := 60 * time.Second

	for {
		err := m.runWorkerOnce(ctx, w)
		if ctx.Err() != nil {
			// Graceful shutdown — close open episode
			if w.episode != nil {
				m.closeEpisode(w, "process_shutdown", "interrupted")
			}
			w.setState(StateStopped)
			return
		}
		if w.episode != nil {
			m.closeEpisode(w, "worker_error", "interrupted")
		}

		// Unsupported cameras should not retry
		w.stateMu.RLock()
		st := w.state
		w.stateMu.RUnlock()
		if st == StateUnsupported || st == StateMisconfigured {
			log.Printf("motion cam %d: %s — not retrying", w.cameraID, st)
			return
		}

		w.setState(StateDisconnected)
		log.Printf("motion cam %d: %v, retrying in %v", w.cameraID, err, backoff)

		m.hub.Publish(model.MotionEvent{
			Type:        "motion.error",
			CameraID:    w.cameraID,
			TimestampMs: time.Now().UnixMilli(),
			Error:       err.Error(),
		})

		select {
		case <-ctx.Done():
			w.setState(StateStopped)
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxBackoff)
	}
}

func (m *Manager) runWorkerOnce(ctx context.Context, w *worker) error {
	// Read camera credentials
	var ip, onvifPath, username, password, camName string
	var port int
	err := m.db.QueryRow(
		`SELECT ip, port, onvif_path, username, password, name FROM cameras WHERE id = ? AND enabled = 1`,
		w.cameraID,
	).Scan(&ip, &port, &onvifPath, &username, &password, &camName)
	if err != nil {
		w.setState(StateMisconfigured)
		return err
	}

	// Create PullPoint subscription
	session, err := onvif.CreatePullPoint(ip, port, onvifPath, username, password)
	if err != nil {
		if isUnsupported(err) {
			w.setState(StateUnsupported)
		} else {
			w.setState(StateDegraded)
		}
		return err
	}
	defer session.Unsubscribe()

	w.setState(StateActive)
	log.Printf("motion cam %d (%s): PullPoint subscription active", w.cameraID, camName)

	m.hub.Publish(model.MotionEvent{
		Type:        "motion.runtime",
		CameraID:    w.cameraID,
		CameraName:  camName,
		TimestampMs: time.Now().UnixMilli(),
		State:       string(StateActive),
	})

	// Restore any open episode from DB (in case of fast restart)
	if ep, err := m.store.OpenEpisode(w.cameraID); err == nil && ep != nil {
		w.episode = &openEpisode{id: ep.ID, lastSeenMs: ep.LastSeenAtMs}
	}

	// Pull loop
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if session.NeedsRenew() {
			if err := session.Renew(); err != nil {
				log.Printf("motion cam %d: renew failed: %v", w.cameraID, err)
			}
		}

		notifications, err := session.Pull(ctx)
		if err != nil {
			return err
		}

		nowMs := time.Now().UnixMilli()

		for _, n := range notifications {
			m.processNotification(w, camName, n, nowMs)
		}

		// Check idle timeout for open episodes
		if w.episode != nil && (nowMs-w.episode.lastSeenMs) > idleTimeoutMs {
			m.closeEpisode(w, "idle_timeout", "inferred_closed")
		}
	}
}

// ---------------------------------------------------------------------------
// Episode state machine

func (m *Manager) processNotification(w *worker, camName string, n onvif.MotionNotification, nowMs int64) {
	var cameraMs *int64
	if !n.CameraTime.IsZero() {
		ms := n.CameraTime.UnixMilli()
		cameraMs = &ms
	}

	if n.Active {
		if w.episode == nil {
			// Open new episode
			ep := &model.MotionEpisode{
				CameraID:      w.cameraID,
				Source:        "onvif",
				StartedAtMs:   nowMs,
				LastSeenAtMs:  nowMs,
				Topic:         n.Topic,
				RuleName:      n.RuleName,
				SourceToken:   n.SourceToken,
				FirstCameraMs: cameraMs,
			}
			if err := m.store.InsertEpisode(ep); err != nil {
				log.Printf("motion cam %d: insert episode: %v", w.cameraID, err)
				return
			}
			w.episode = &openEpisode{id: ep.ID, lastSeenMs: nowMs}
			m.hub.Publish(model.MotionEvent{
				Type:        "motion.start",
				CameraID:    w.cameraID,
				CameraName:  camName,
				EpisodeID:   ep.ID,
				TimestampMs: nowMs,
			})
		} else {
			// Bump existing episode
			w.episode.lastSeenMs = nowMs
			if err := m.store.BumpEpisode(w.episode.id, nowMs, cameraMs); err != nil {
				log.Printf("motion cam %d: bump episode: %v", w.cameraID, err)
			}
			m.hub.Publish(model.MotionEvent{
				Type:        "motion.update",
				CameraID:    w.cameraID,
				CameraName:  camName,
				EpisodeID:   w.episode.id,
				TimestampMs: nowMs,
			})
		}
	} else {
		// Motion inactive
		if w.episode != nil {
			m.closeEpisode(w, "inactive", "closed")
		}
	}
}

func (m *Manager) closeEpisode(w *worker, reason, status string) {
	if w.episode == nil {
		return
	}
	nowMs := time.Now().UnixMilli()
	if err := m.store.CloseEpisode(w.episode.id, nowMs, status, reason); err != nil {
		log.Printf("motion cam %d: close episode %d: %v", w.cameraID, w.episode.id, err)
	}
	dur := nowMs - w.episode.lastSeenMs
	if dur < 0 {
		dur = 0
	}
	// Read started_at_ms for accurate duration
	var startMs int64
	_ = m.db.QueryRow(`SELECT started_at_ms FROM motion_episodes WHERE id = ?`, w.episode.id).Scan(&startMs)
	if startMs > 0 {
		dur = nowMs - startMs
	}

	// Look up camera name for event
	var camName string
	_ = m.db.QueryRow(`SELECT name FROM cameras WHERE id = ?`, w.cameraID).Scan(&camName)

	m.hub.Publish(model.MotionEvent{
		Type:        "motion.end",
		CameraID:    w.cameraID,
		CameraName:  camName,
		EpisodeID:   w.episode.id,
		TimestampMs: nowMs,
		DurationMs:  &dur,
	})
	w.episode = nil
}

// ---------------------------------------------------------------------------
// Cleanup

func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			n, err := m.store.DeleteExpired(time.Now().UnixMilli(), cleanupBatch)
			if err != nil {
				log.Printf("motion cleanup: %v", err)
			} else if n > 0 {
				log.Printf("motion cleanup: deleted %d expired episodes", n)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers

func (w *worker) setState(s WorkerState) {
	w.stateMu.Lock()
	w.state = s
	w.stateMu.Unlock()
}

func isUnsupported(err error) bool {
	s := err.Error()
	return strings.Contains(s, "does not advertise Events") ||
		strings.Contains(s, "not implemented") ||
		strings.Contains(s, "ActionNotSupported")
}
