// Package events provides an in-process, strictly user_id-scoped fan-out hub and
// an SSE handler for delivering real-time file events to authenticated clients.
//
// The hub holds no background goroutine: Publish and Subscribe operate directly
// under a sync.RWMutex, so there is no server-lifecycle machinery to manage. Its
// single hard invariant is the per-user_id fan-out boundary — an event published
// for one user is never delivered to another.
package events

import (
	"sync"
	"time"
)

const (
	// heartbeatInterval is how often the SSE handler emits a ": ping" comment to
	// keep the connection (and any intermediary proxies) from idling out. 15s is
	// chosen to beat the common ~25s TCP keepalive window that otherwise drops
	// otherwise-idle streams.
	heartbeatInterval = 15 * time.Second

	// subscriberBuffer bounds each subscriber's channel. When the buffer is full,
	// Publish drops the event for that one slow subscriber rather than blocking
	// the caller or starving other subscribers.
	subscriberBuffer = 16
)

// Event is the payload delivered on the SSE stream. The JSON tags define the
// wire contract consumed by the CLI auto-refresh subscriber; keep them stable.
type Event struct {
	Type        string `json:"type"` // e.g. "file.uploaded"
	FileID      string `json:"id"`
	Filename    string `json:"filename"`
	Size        *int64 `json:"size"`
	ContentType string `json:"contentType"`
	CreatedAt   string `json:"createdAt"` // RFC3339
}

// Publisher is the one-method seam the API layer depends on, so callers stay
// decoupled from the concrete hub and can inject a spy in tests. *Hub satisfies
// it.
type Publisher interface {
	Publish(userID string, ev Event)
}

type subscriber struct {
	ch chan Event
}

// Hub is an in-process, strictly user_id-scoped fan-out registry.
type Hub struct {
	mu   sync.RWMutex
	subs map[string]map[*subscriber]struct{}
}

// NewHub returns an empty Hub ready to accept subscribers.
func NewHub() *Hub {
	return &Hub{subs: make(map[string]map[*subscriber]struct{})}
}

// Subscribe registers a new subscriber for userID and returns its receive-only
// event channel plus an idempotent unsubscribe func.
//
// The hub owns the channel's close: unsubscribe removes the subscriber and closes
// the channel under the write lock. Because Publish holds the read lock across its
// send, a close can never race a send (the two locks are mutually exclusive), so
// there is no send-on-closed-channel panic. The unsubscribe is wrapped in
// sync.Once, so a double call (e.g. a deferred call plus an explicit one) is safe.
func (h *Hub) Subscribe(userID string) (<-chan Event, func()) {
	sub := &subscriber{ch: make(chan Event, subscriberBuffer)}

	h.mu.Lock()
	if h.subs[userID] == nil {
		h.subs[userID] = make(map[*subscriber]struct{})
	}
	h.subs[userID][sub] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			if set, ok := h.subs[userID]; ok {
				delete(set, sub)
				if len(set) == 0 {
					delete(h.subs, userID)
				}
			}
			close(sub.ch)
		})
	}
	return sub.ch, unsubscribe
}

// Publish delivers ev to every subscriber registered under userID. It is
// non-blocking: a subscriber whose buffer is full simply drops this event,
// without blocking the caller or any other subscriber. Publish touches only
// subs[userID] — that scoping is the per-user fan-out boundary.
func (h *Hub) Publish(userID string, ev Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for sub := range h.subs[userID] {
		select {
		case sub.ch <- ev:
		default: // drop-not-block: a slow consumer loses this event
		}
	}
}

// subscriberCount reports how many subscribers are currently registered for
// userID. It exists for tests to assert the leak-free unsubscribe behaviour.
func (h *Hub) subscriberCount(userID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs[userID])
}
