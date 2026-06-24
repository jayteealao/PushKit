package events

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pushkit/backend/internal/auth"
)

// Handler serves the authenticated SSE stream at GET /v1/events. Each connection
// subscribes to the hub under the request's authenticated user_id and streams
// that user's events until the client disconnects.
type Handler struct {
	pub Subscriber
}

// Subscriber is the seam the Handler depends on: it can subscribe a user_id and
// receive events from the hub without depending on the concrete *Hub type.
type Subscriber interface {
	Subscribe(userID string) (<-chan Event, func())
}

// NewHandler constructs a Handler backed by the given Subscriber.
func NewHandler(sub Subscriber) *Handler {
	return &Handler{pub: sub}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Disable reverse-proxy response buffering (nginx and friends) so frames are
	// delivered immediately rather than held until the buffer fills.
	w.Header().Set("X-Accel-Buffering", "no")

	rc := http.NewResponseController(w)

	ch, unsub := h.pub.Subscribe(userID)
	defer unsub()

	// Flush the headers immediately so the client sees an open stream before any
	// event arrives. A flush error means the connection cannot stream (no
	// underlying flusher, or the client is already gone) — nothing more to do.
	if err := rc.Flush(); err != nil {
		return
	}

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			// Client disconnected; the deferred unsub removes the subscriber.
			return
		case ev, ok := <-ch:
			if !ok {
				return // subscription closed
			}
			if !writeEvent(w, rc, ev) {
				return // client gone
			}
		case <-ticker.C:
			if !writeHeartbeat(w, rc) {
				return // client gone
			}
		}
	}
}

// writeEvent emits a single named SSE frame ("event: <type>\ndata: <json>\n\n")
// and flushes it. A named event keeps heartbeat comment lines invisible to
// EventSource "file.uploaded" listeners. It returns false when the write or flush
// fails, signalling the caller to tear the stream down. A malformed event (which
// the Event struct cannot realistically produce) is skipped without ending the
// stream.
func writeEvent(w io.Writer, rc *http.ResponseController, ev Event) bool {
	data, err := json.Marshal(ev)
	if err != nil {
		return true
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, data); err != nil {
		return false
	}
	return rc.Flush() == nil
}

// writeHeartbeat emits a ": ping" SSE comment line and flushes it. Comment lines
// are ignored by EventSource but keep the connection and intermediary proxies
// alive. It returns false when the write or flush fails.
func writeHeartbeat(w io.Writer, rc *http.ResponseController) bool {
	if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
		return false
	}
	return rc.Flush() == nil
}
