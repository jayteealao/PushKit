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

// subscribingHub wraps *Hub and signals on subscribeReady each time Subscribe is
// called, giving tests a deterministic "subscriber is registered" signal without
// polling or sleeping.
type subscribingHub struct {
	*Hub
	subscribeReady chan struct{}
}

func newSubscribingHub() *subscribingHub {
	return &subscribingHub{
		Hub:            NewHub(),
		subscribeReady: make(chan struct{}, 8),
	}
}

func (s *subscribingHub) Subscribe(userID string) (<-chan Event, func()) {
	ch, unsub := s.Hub.Subscribe(userID)
	s.subscribeReady <- struct{}{}
	return ch, unsub
}

// TestHandler_StreamsScopedEventAndLeaksNothing exercises the SSE handler end to
// end over a real HTTP connection: it authenticates, receives a published
// file.uploaded frame, and — the leak gate — releases its subscriber slot when
// the client disconnects.
func TestHandler_StreamsScopedEventAndLeaksNothing(t *testing.T) {
	sh := newSubscribingHub()
	cfg := &config.Config{APIKeyMap: map[string]string{"key-a": "userA"}}

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(cfg))
		r.Handle("/v1/events", NewHandler(sh))
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

	// Wait for the subscribe signal — fired by subscribingHub.Subscribe before
	// the handler returns 200, so it is already buffered by the time Do returns.
	waitForSubscribe(t, sh)

	sh.Hub.Publish("userA", Event{Type: "file.uploaded", FileID: "f1", Filename: "report.pdf"})

	frame := readFrame(t, resp.Body)
	if !strings.Contains(frame, "event: file.uploaded") {
		t.Fatalf("frame missing named event:\n%s", frame)
	}
	if !strings.Contains(frame, `"id":"f1"`) {
		t.Fatalf("frame missing event data:\n%s", frame)
	}

	// Leak gate: once the client disconnects, the handler must unsubscribe.
	cancel()
	waitForCount(t, sh.Hub, "userA", 0)
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

// waitForSubscribe blocks until sh.subscribeReady fires or testWait elapses.
// It replaces sleep-based polling for the "subscriber registered" transition.
func waitForSubscribe(t *testing.T, sh *subscribingHub) {
	t.Helper()
	select {
	case <-sh.subscribeReady:
	case <-time.After(testWait):
		t.Fatal("Subscribe was not called within deadline")
	}
}

// waitForCount blocks until hub.subscriberCount(userID) == want or testWait
// elapses. The unsubscribe transition happens asynchronously (the context
// cancellation is processed in the handler goroutine), so we must observe it
// with a bounded wait rather than an instant check. We check at 1 ms intervals
// — fast enough to be deterministic on any CI runner, cheap enough not to spin.
func waitForCount(t *testing.T, hub *Hub, userID string, want int) {
	t.Helper()
	timeout := time.After(testWait)
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if hub.subscriberCount(userID) == want {
				return
			}
		case <-timeout:
			t.Fatalf("subscriberCount(%q) = %d, want %d within %s",
				userID, hub.subscriberCount(userID), want, testWait)
		}
	}
}

// TestHandler_TwoUserIsolation_AuthToPublish is the handler-level security gate:
// an event published for userA must reach A's SSE stream and must NOT reach
// userB's stream. It wires the real auth middleware, connects two users, and
// uses channel signals — not sleep — for all synchronisation.
func TestHandler_TwoUserIsolation_AuthToPublish(t *testing.T) {
	sh := newSubscribingHub()
	cfg := &config.Config{APIKeyMap: map[string]string{
		"key-a": "userA",
		"key-b": "userB",
	}}

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(cfg))
		r.Handle("/v1/events", NewHandler(sh))
	})
	srv := httptest.NewServer(r)
	defer srv.Close()

	ctxA, cancelA := context.WithCancel(context.Background())
	defer cancelA()
	ctxB, cancelB := context.WithCancel(context.Background())
	defer cancelB()

	// Connect userA.
	reqA, err := http.NewRequestWithContext(ctxA, http.MethodGet, srv.URL+"/v1/events", nil)
	if err != nil {
		t.Fatalf("build request A: %v", err)
	}
	reqA.Header.Set("X-API-Key", "key-a")
	respA, err := http.DefaultClient.Do(reqA)
	if err != nil {
		t.Fatalf("connect A: %v", err)
	}
	defer respA.Body.Close()

	// Wait for A to be subscribed before connecting B.
	waitForSubscribe(t, sh)

	// Connect userB.
	reqB, err := http.NewRequestWithContext(ctxB, http.MethodGet, srv.URL+"/v1/events", nil)
	if err != nil {
		t.Fatalf("build request B: %v", err)
	}
	reqB.Header.Set("X-API-Key", "key-b")
	respB, err := http.DefaultClient.Do(reqB)
	if err != nil {
		t.Fatalf("connect B: %v", err)
	}
	defer respB.Body.Close()

	// Wait for B to be subscribed.
	waitForSubscribe(t, sh)

	if respA.StatusCode != http.StatusOK {
		t.Fatalf("A status = %d, want 200", respA.StatusCode)
	}
	if respB.StatusCode != http.StatusOK {
		t.Fatalf("B status = %d, want 200", respB.StatusCode)
	}

	// Publish an event for userA only.
	sh.Hub.Publish("userA", Event{Type: "file.uploaded", FileID: "fa1", Filename: "a.pdf"})

	// A must receive the event.
	frameA := readFrame(t, respA.Body)
	if !strings.Contains(frameA, "event: file.uploaded") {
		t.Fatalf("userA frame missing named event:\n%s", frameA)
	}
	if !strings.Contains(frameA, `"id":"fa1"`) {
		t.Fatalf("userA frame missing event data:\n%s", frameA)
	}

	// B must NOT receive anything. We send a second event to B's stream as a
	// sentinel: if B's channel had received A's event it would come out before
	// the sentinel. The sentinel confirms B's stream is live and its silence on
	// A's event is meaningful, not just a stalled connection.
	sh.Hub.Publish("userB", Event{Type: "file.uploaded", FileID: "sentinel", Filename: "sentinel.pdf"})
	frameB := readFrame(t, respB.Body)
	if strings.Contains(frameB, `"id":"fa1"`) {
		t.Fatalf("cross-user leak: userB received userA's event:\n%s", frameB)
	}
	if !strings.Contains(frameB, `"id":"sentinel"`) {
		t.Fatalf("userB sentinel event not received (stream may be broken):\n%s", frameB)
	}

	// Leak gate: disconnect both clients, assert clean unsubscribe.
	cancelA()
	waitForCount(t, sh.Hub, "userA", 0)
	cancelB()
	waitForCount(t, sh.Hub, "userB", 0)
}
