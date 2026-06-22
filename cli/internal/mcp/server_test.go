package mcp

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/pushkit/cli/internal/client"
)

// TestNewConstructsWithoutCredentials proves the server starts even with no API
// key (start-and-error): construction registers all four tools and never panics,
// even though schema inference for every tool input runs inside AddTool.
func TestNewConstructsWithoutCredentials(t *testing.T) {
	s := New(Options{
		Client:   &stubClient{},
		APIKey:   "", // missing on purpose
		State:    LoadState(t.TempDir(), "http://localhost:8080"),
		StateDir: t.TempDir(),
	})
	if s == nil || s.mcp == nil {
		t.Fatal("server was not constructed")
	}
}

// TestToolsErrorClearlyWithoutKey is AC#5: with no key, each tool returns a
// clear, actionable error rather than crashing.
func TestToolsErrorClearlyWithoutKey(t *testing.T) {
	h := &handlers{c: &stubClient{}, apiKey: ""}
	ctx := context.Background()

	checks := []struct {
		name string
		call func() error
	}{
		{"list", func() error { _, _, err := h.list(ctx, nil, listInput{}); return err }},
		{"push", func() error { _, _, err := h.push(ctx, nil, pushInput{Paths: []string{"x"}}); return err }},
		{"pull", func() error { _, _, err := h.pull(ctx, nil, pullInput{Newest: true}); return err }},
		{"delete", func() error { _, _, err := h.delete(ctx, nil, deleteInput{ID: "x"}); return err }},
	}
	for _, c := range checks {
		err := c.call()
		if err == nil {
			t.Errorf("%s: expected an auth error with no key", c.name)
			continue
		}
		if !strings.Contains(err.Error(), "API key") {
			t.Errorf("%s: error %q should mention the missing API key", c.name, err)
		}
	}
}

// TestUnreachableBackendIsToolError shows a connection failure surfaces as a
// readable tool error (the SDK packs a returned error into the tool result).
func TestUnreachableBackendIsToolError(t *testing.T) {
	stub := &stubClient{
		listFn: func(_ context.Context, _ string, _ int, _, _, _ string) (*client.FileListResponse, error) {
			return nil, fmt.Errorf("request failed: dial tcp: connection refused")
		},
	}
	h := &handlers{c: stub, apiKey: "k"}

	_, _, err := h.list(context.Background(), nil, listInput{})
	if err == nil || !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("expected a clear connection error, got %v", err)
	}
}

// TestServerListsFourToolsOverSession drives a real MCP client against the server
// over the SDK's in-memory transport: it performs the initialize handshake and
// tools/list, asserting exactly the four file tools are advertised. This is the
// deterministic runtime-truth evidence that the server registers and works (no
// flaky stdio subprocess), and guards against a tool being dropped from wiring.
func TestServerListsFourToolsOverSession(t *testing.T) {
	srv := New(Options{
		Client:   &stubClient{},
		APIKey:   "k",
		State:    LoadState(t.TempDir(), "http://localhost:8080"),
		StateDir: t.TempDir(),
	})
	ctx := context.Background()

	clientT, serverT := mcp.NewInMemoryTransports()
	ss, err := srv.mcp.Connect(ctx, serverT, nil) // the server must connect first
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer ss.Close()

	c := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil)
	cs, err := c.Connect(ctx, clientT, nil) // performs the initialize handshake
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	got := map[string]bool{}
	for _, tool := range res.Tools {
		got[tool.Name] = true
	}
	for _, want := range []string{"pushkit_push", "pushkit_pull", "pushkit_list", "pushkit_delete"} {
		if !got[want] {
			t.Errorf("tool %q not advertised; got %v", want, got)
		}
	}
	if len(res.Tools) != 4 {
		t.Errorf("expected exactly 4 tools, got %d: %v", len(res.Tools), got)
	}
}
