package mcp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/pushkit/cli/internal/client"
)

// isRegistered reports whether a file ID is currently exposed as a concrete
// resource (test helper — same package).
func (s *Server) isRegistered(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.registered[id]
	return ok
}

func listChangedNotifier() (*mcp.ClientOptions, chan struct{}) {
	ch := make(chan struct{}, 8)
	return &mcp.ClientOptions{
		ResourceListChangedHandler: func(context.Context, *mcp.ResourceListChangedRequest) {
			select {
			case ch <- struct{}{}:
			default:
			}
		},
	}, ch
}

func assertNotified(t *testing.T, ch <-chan struct{}, within time.Duration) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(within):
		t.Fatal("expected a resources/list_changed notification, got none")
	}
}

func assertNoNotification(t *testing.T, ch <-chan struct{}, within time.Duration) {
	t.Helper()
	select {
	case <-ch:
		t.Fatal("expected NO notification, but one was delivered")
	case <-time.After(within):
	}
}

func waitFor(t *testing.T, within time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("condition not met within deadline")
}

// swapBackoff sets per-instance reconnect parameters on s and returns a restore
// func. The restore writes s.reconnectBase/Max, which the subscriber goroutine
// reads, so a caller that starts runSubscriber must stop and JOIN that goroutine
// before the deferred restore runs (see TestSubscriber_ReconnectReconcilesNewFiles);
// otherwise the restore races the goroutine's reads at teardown.
func swapBackoff(s *Server, base, max time.Duration) func() {
	ob, om := s.reconnectBase, s.reconnectMax
	s.reconnectBase, s.reconnectMax = base, max
	return func() { s.reconnectBase, s.reconnectMax = ob, om }
}

func hasResourceURI(rs []*mcp.Resource, uri string) bool {
	for _, r := range rs {
		if r.URI == uri {
			return true
		}
	}
	return false
}

// TestScanSSE_ParsesFramesAndIgnoresHeartbeats covers the frame parser: event
// types, a heartbeat comment line, and a data line larger than the default 64KB
// scanner token (proving the buffer was raised).
func TestScanSSE_ParsesFramesAndIgnoresHeartbeats(t *testing.T) {
	big := strings.Repeat("x", 70*1024) // > 64KB default scanner token limit
	raw := ": ping\n\n" +
		"event: file.uploaded\ndata: {\"id\":\"a\"}\n\n" +
		"event: file.uploaded\ndata: " + big + "\n\n" +
		"event: other\ndata: {\"id\":\"b\"}\n\n"

	type frame struct{ et, data string }
	var got []frame
	err := scanSSE(context.Background(), strings.NewReader(raw), func(et, d string) {
		got = append(got, frame{et, d})
	})
	if !errors.Is(err, io.EOF) {
		t.Fatalf("scanSSE error = %v, want io.EOF at clean end", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d frames, want 3 (heartbeat must be ignored): %+v", len(got), got)
	}
	if got[0].et != "file.uploaded" || got[0].data != `{"id":"a"}` {
		t.Errorf("frame 0 = %+v", got[0])
	}
	if got[1].data != big {
		t.Errorf("frame 1 data truncated: len=%d, want %d", len(got[1].data), len(big))
	}
	if got[2].et != "other" {
		t.Errorf("frame 2 event type = %q, want other", got[2].et)
	}
}

// TestDispatch_SelfEchoSuppressedThirdPartyRefreshes is AC#3: a file this server
// just pushed is suppressed (no re-list, no notification); a third-party upload
// still refreshes.
func TestDispatch_SelfEchoSuppressedThirdPartyRefreshes(t *testing.T) {
	var listCalls int32
	s := newTestServer(t, &stubClient{
		listFn: func(_ context.Context, _ string, _ int, _, _, _ string) (*client.FileListResponse, error) {
			atomic.AddInt32(&listCalls, 1)
			return &client.FileListResponse{Items: []client.FileResponse{
				{ID: "idY", OriginalFilename: "y.txt"},
			}}, nil
		},
	})
	ctx := context.Background()
	opts, notified := listChangedNotifier()
	connectClient(t, ctx, s, opts)

	// Self-pushed ID → suppressed: no re-list, no notification.
	s.h.state.RecordPushed("idX")
	s.dispatch(ctx, "file.uploaded", `{"type":"file.uploaded","id":"idX"}`)
	if got := atomic.LoadInt32(&listCalls); got != 0 {
		t.Fatalf("self-echo must not re-list; ListFiles called %d times", got)
	}
	assertNoNotification(t, notified, 100*time.Millisecond)

	// Third-party ID → refresh: re-list + list_changed.
	s.dispatch(ctx, "file.uploaded", `{"type":"file.uploaded","id":"idY"}`)
	assertNotified(t, notified, time.Second)
	if got := atomic.LoadInt32(&listCalls); got != 1 {
		t.Errorf("third-party event should re-list exactly once; got %d", got)
	}
}

// TestDispatch_RefreshDeliversListChangedAndResource is the AC#1 in-process
// evidence: a file.uploaded delivers a resources/list_changed and the new file
// appears in the client's resource list — the no-re-ask refresh, deterministically.
func TestDispatch_RefreshDeliversListChangedAndResource(t *testing.T) {
	s := newTestServer(t, &stubClient{
		listFn: func(_ context.Context, _ string, _ int, _, _, _ string) (*client.FileListResponse, error) {
			return &client.FileListResponse{Items: []client.FileResponse{
				{ID: "new-1", OriginalFilename: "report.pdf", ContentType: "application/pdf"},
			}}, nil
		},
	})
	ctx := context.Background()
	opts, notified := listChangedNotifier()
	cs := connectClient(t, ctx, s, opts)

	s.dispatch(ctx, "file.uploaded", `{"type":"file.uploaded","id":"new-1"}`)
	assertNotified(t, notified, time.Second)

	res, err := cs.ListResources(ctx, nil)
	if err != nil {
		t.Fatalf("list resources: %v", err)
	}
	if !hasResourceURI(res.Resources, fileURI("new-1")) {
		t.Errorf("resource %s not in client list after refresh", fileURI("new-1"))
	}
}

// TestSubscriber_ReconnectReconcilesNewFiles is AC#4: after the stream drops and
// the subscriber reconnects, it reconciles by re-listing and picks up a file that
// appeared while disconnected (no missed-event replay assumed).
func TestSubscriber_ReconnectReconcilesNewFiles(t *testing.T) {
	// SSE origin that accepts the connection then closes immediately, forcing a
	// reconnect each time.
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// Return immediately → body EOF → subscriber reconnects.
	}))
	defer origin.Close()

	var listCalls int32
	stub := &stubClient{
		listFn: func(_ context.Context, _ string, _ int, _, _, _ string) (*client.FileListResponse, error) {
			n := atomic.AddInt32(&listCalls, 1)
			items := []client.FileResponse{{ID: "a", OriginalFilename: "a.txt"}}
			if n >= 2 { // "b" appears only from the second reconcile onward
				items = append(items, client.FileResponse{ID: "b", OriginalFilename: "b.txt"})
			}
			return &client.FileListResponse{Items: items}, nil
		},
	}
	s := New(Options{
		Client:   stub,
		APIKey:   "k",
		BaseURL:  origin.URL,
		State:    LoadState(t.TempDir(), origin.URL),
		StateDir: t.TempDir(),
	})
	// Set fast reconnect on the instance before starting the goroutine — no global
	// mutation, so parallel tests are hermetic and there is no data race.
	defer swapBackoff(s, 5*time.Millisecond, 20*time.Millisecond)()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { s.runSubscriber(ctx); close(done) }()

	waitFor(t, 2*time.Second, func() bool { return s.isRegistered("b") })
	if !s.isRegistered("a") {
		t.Error("file a should remain registered across reconnects")
	}

	// Stop the subscriber and wait for it to return before the test does: the
	// deferred swapBackoff restore writes s.reconnectBase/Max, which the goroutine
	// reads — joining here removes that teardown race (and the goroutine leak).
	cancel()
	<-done
}

// TestSubscriber_StopsOnContextCancel is the leak gate: the subscriber goroutine
// returns promptly when its context is cancelled while blocked on the stream.
func TestSubscriber_StopsOnContextCancel(t *testing.T) {
	var connected int32
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		atomic.StoreInt32(&connected, 1)
		<-r.Context().Done() // hold the stream open until the client goes away
	}))
	defer origin.Close()

	s := New(Options{
		Client:   &stubClient{}, // nil listFn → empty list; reconcile registers nothing
		APIKey:   "k",
		BaseURL:  origin.URL,
		State:    LoadState(t.TempDir(), origin.URL),
		StateDir: t.TempDir(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.runSubscriber(ctx); close(done) }()

	waitFor(t, time.Second, func() bool { return atomic.LoadInt32(&connected) == 1 })
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runSubscriber did not return after context cancel")
	}
}
