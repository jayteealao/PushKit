package mcp

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// selfEchoWindow suppresses the refresh echo for files this server pushed within
// the window. The backend event carries no source field, so de-dup is keyed on
// the file ID alone.
const selfEchoWindow = 30 * time.Second

// sseMaxLineBytes raises the bufio.Scanner token limit above its 64KB default so
// a large data: line is not silently truncated.
const sseMaxLineBytes = 1 << 20 // 1 MiB

// newSSEClient constructs the http.Client used for the long-lived SSE stream.
// It must NOT set an absolute Timeout (that is a deadline from request start and
// would sever the stream); cancellation is via context and dead peers are caught
// by TCP keep-alive. ResponseHeaderTimeout gates only the initial response headers.
// TLS is floored at 1.2 as defense-in-depth.
func newSSEClient() *http.Client {
	return &http.Client{
		Timeout: 0,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}
}

// fileEvent is the JSON payload carried by a file.uploaded SSE frame.
type fileEvent struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// runSubscriber keeps an SSE subscription to GET /v1/events alive for the life of
// ctx, reconnecting with exponential backoff + jitter. It is best-effort: errors
// are logged to stderr and retried, and it never affects tool calls. It returns
// when ctx is cancelled.
func (s *Server) runSubscriber(ctx context.Context) {
	delay := s.reconnectBase
	for {
		err := s.subscribeOnce(ctx, func() { delay = s.reconnectBase })
		if ctx.Err() != nil {
			return
		}
		if err != nil && !errors.Is(err, io.EOF) {
			fmt.Fprintf(os.Stderr, "pushkit mcp: event stream: %v (retrying in ~%s)\n", err, delay)
		}
		select {
		case <-time.After(jitter(delay)):
		case <-ctx.Done():
			return
		}
		if delay *= 2; delay > s.reconnectMax {
			delay = s.reconnectMax
		}
	}
}

// jitter returns d scaled by a random factor in [0.5, 1.5) to spread reconnects.
// If d is zero or negative, 0 is returned (guards against rand.Int64N(0) panic).
func jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	return d/2 + time.Duration(rand.Int64N(int64(d)))
}

// subscribeOnce reconciles the resource set, opens one SSE connection, and reads
// it until the stream ends or ctx is cancelled. Reconciling first covers both
// initial population and post-reconnect catch-up — the backend has no replay /
// Last-Event-ID, so a fresh full re-list is the reconnection contract. onConnect
// is invoked once the stream is established (used to reset the backoff).
func (s *Server) subscribeOnce(ctx context.Context, onConnect func()) error {
	if err := s.reconcileResources(ctx); err != nil {
		return fmt.Errorf("reconcile: %w", err)
	}

	endpoint := strings.TrimRight(s.baseURL, "/") + "/v1/events"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", s.h.apiKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := s.sseClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("event stream returned HTTP %d", resp.StatusCode)
	}
	onConnect()
	return scanSSE(ctx, resp.Body, func(eventType, data string) {
		s.dispatch(ctx, eventType, data)
	})
}

// scanSSE parses a text/event-stream body, invoking emit(eventType, data) once
// per complete frame (terminated by a blank line). Comment/heartbeat lines (":")
// are ignored. It returns io.EOF on a clean stream end and ctx.Err() on
// cancellation, so the caller can reconnect.
func scanSSE(ctx context.Context, body io.Reader, emit func(eventType, data string)) error {
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 0, 64*1024), sseMaxLineBytes)

	var eventType string
	var data strings.Builder
	for sc.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := sc.Text()
		switch {
		case line == "":
			if eventType != "" || data.Len() > 0 {
				emit(eventType, data.String())
			}
			eventType = ""
			data.Reset()
		case strings.HasPrefix(line, ":"):
			// comment / heartbeat — ignore
		case strings.HasPrefix(line, "event:"):
			eventType = strings.TrimSpace(line[len("event:"):])
		case strings.HasPrefix(line, "data:"):
			if data.Len() > 0 {
				data.WriteByte('\n') // SSE joins multiple data lines with newlines
			}
			data.WriteString(strings.TrimPrefix(line[len("data:"):], " "))
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return io.EOF
}

// dispatch handles one parsed SSE frame. It refreshes the resource set on a
// file.uploaded — unless the file was just pushed by this server (self-echo
// de-dup), in which case the redundant refresh is suppressed.
func (s *Server) dispatch(ctx context.Context, eventType, data string) {
	if eventType != "file.uploaded" || data == "" {
		return
	}
	var ev fileEvent
	if err := json.Unmarshal([]byte(data), &ev); err != nil {
		fmt.Fprintf(os.Stderr, "pushkit mcp: malformed event payload: %v\n", err)
		return
	}
	if ev.ID == "" {
		return
	}
	if s.h.state != nil && s.h.state.WasRecentlyPushed(ev.ID, selfEchoWindow) {
		return // this server pushed it moments ago — suppress the echo
	}
	if err := s.reconcileResources(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "pushkit mcp: refresh after %s failed: %v\n", ev.ID, err)
	}
}
