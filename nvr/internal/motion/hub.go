package motion

import (
	"sync"

	"nvr/internal/model"
)

// Hub is a backpressure-safe pub/sub for live motion events.
// Subscribers receive events on bounded channels; slow consumers have events dropped.
type Hub struct {
	mu       sync.RWMutex
	perCam   map[int64]map[chan model.MotionEvent]struct{}
	global   map[chan model.MotionEvent]struct{}
}

// NewHub creates a Hub.
func NewHub() *Hub {
	return &Hub{
		perCam: make(map[int64]map[chan model.MotionEvent]struct{}),
		global: make(map[chan model.MotionEvent]struct{}),
	}
}

// Subscribe returns a channel that receives motion events.
// cameraID=0 subscribes to all cameras (global).
func (h *Hub) Subscribe(cameraID int64) chan model.MotionEvent {
	ch := make(chan model.MotionEvent, 64)
	h.mu.Lock()
	defer h.mu.Unlock()
	if cameraID == 0 {
		h.global[ch] = struct{}{}
	} else {
		if h.perCam[cameraID] == nil {
			h.perCam[cameraID] = make(map[chan model.MotionEvent]struct{})
		}
		h.perCam[cameraID][ch] = struct{}{}
	}
	return ch
}

// Unsubscribe removes a channel. cameraID must match the Subscribe call.
func (h *Hub) Unsubscribe(cameraID int64, ch chan model.MotionEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if cameraID == 0 {
		delete(h.global, ch)
	} else {
		delete(h.perCam[cameraID], ch)
		if len(h.perCam[cameraID]) == 0 {
			delete(h.perCam, cameraID)
		}
	}
}

// Publish sends an event to matching subscribers. Non-blocking: slow consumers drop events.
func (h *Hub) Publish(evt model.MotionEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.perCam[evt.CameraID] {
		select {
		case ch <- evt:
		default:
		}
	}
	for ch := range h.global {
		select {
		case ch <- evt:
		default:
		}
	}
}
