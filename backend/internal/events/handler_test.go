package events

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/pushkit/backend/internal/auth"
	"github.com/pushkit/backend/internal/config"
)

// TestHandler_StreamsScopedEventAndLeaksNothing exercises the SSE handler end to
// end over a real HTTP connection: it authenticates, receives a published
// file.uploaded frame, and — the leak gate — releases its subscriber slot when
// the client disconnects.
func TestHandler_StreamsScopedEventAndLeaksNothing(t *testing.T) {
	hub := NewHub()
	cfg := &config.Config{APIKeyMap: map[string]string{"key-a": "userA"}}

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(cfg))
		r.Handle("/v1/events", &Handler{Hub: hub})
	})
	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/v1/events", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("X-API-Key", "key-a")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}

	// The handler subscribes before flushing headers, so the subscriber is
	// registered by the time Do returns 200.
	waitForCount(t, hub, "userA", 1)

	hub.Publish("userA", Event{Type: "file.uploaded", FileID: "f1", Filename: "report.pdf"})

	frame := readFrame(t, resp.Body)
	if !strings.Contains(frame, "event: file.uploaded") {
		t.Fatalf("frame missing named event:\n%s", frame)
	}
	if !strings.Contains(frame, `"id":"f1"`) {
		t.Fatalf("frame missing event data:\n%s", frame)
	}

	// Leak gate: once the client disconnects, the handler must unsubscribe.
	cancel()
	waitForCount(t, hub, "userA", 0)
}

// readFrame reads one SSE event frame (lines up to the blank-line terminator),
// skipping ": " comment/heartbeat lines, bounded by testWait so a stalled stream
// fails fast instead of hanging.
func readFrame(t *testing.T, body io.Reader) string {
	t.Helper()

	frames := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(body)
		var b strings.Builder
		for sc.Scan() {
			line := sc.Text()
			if line == "" {
				if b.Len() > 0 {
					frames <- b.String()
					return
				}
				continue
			}
			if strings.HasPrefix(line, ":") {
				continue // heartbeat / comment
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}()

	select {
	case f := <-frames:
		return f
	case <-time.After(testWait):
		t.Fatal("no event frame arrived within deadline")
		return ""
	}
}

// waitForCount polls the hub until userID has want subscribers, failing after
// testWait. The handler registers and unregisters subscribers asynchronously
// relative to the test goroutine, so a bounded poll is the deterministic way to
// observe those transitions.
func waitForCount(t *testing.T, hub *Hub, userID string, want int) {
	t.Helper()
	deadline := time.Now().Add(testWait)
	for {
		got := hub.subscriberCount(userID)
		if got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("subscriberCount(%q) = %d, want %d within %s", userID, got, want, testWait)
		}
		time.Sleep(2 * time.Millisecond)
	}
}
