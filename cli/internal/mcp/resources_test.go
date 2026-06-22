package mcp

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/pushkit/cli/internal/client"
)

// newTestServer builds a Server wired to a stub client, with state in a temp dir
// and no base URL (so the background subscriber stays off — tests drive the
// resource/refresh logic directly).
func newTestServer(t *testing.T, stub *stubClient) *Server {
	t.Helper()
	return New(Options{
		Client:   stub,
		APIKey:   "k",
		State:    LoadState(t.TempDir(), "http://localhost:8080"),
		StateDir: t.TempDir(),
	})
}

func readReq(uri string) *mcp.ReadResourceRequest {
	return &mcp.ReadResourceRequest{Params: &mcp.ReadResourceParams{URI: uri}}
}

func single(t *testing.T, res *mcp.ReadResourceResult) *mcp.ResourceContents {
	t.Helper()
	if res == nil || len(res.Contents) != 1 {
		t.Fatalf("expected exactly one content block, got %+v", res)
	}
	return res.Contents[0]
}

// connectClient drives a real MCP client against the server over the SDK's
// in-memory transport (initialize handshake included) and returns the client
// session. Cleanup is registered via t.Cleanup.
func connectClient(t *testing.T, ctx context.Context, s *Server, opts *mcp.ClientOptions) *mcp.ClientSession {
	t.Helper()
	clientT, serverT := mcp.NewInMemoryTransports()
	ss, err := s.mcp.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { ss.Close() })

	c := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, opts)
	cs, err := c.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func serveContent(t *testing.T, contentType string, body []byte) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// TestReadResource_TextContent: a textual file is returned inline as Text.
func TestReadResource_TextContent(t *testing.T) {
	originURL := serveContent(t, "text/plain; charset=utf-8", []byte("hello world"))
	s := newTestServer(t, &stubClient{
		downloadFn: func(_ context.Context, _ string) (*client.DownloadResponse, error) {
			return &client.DownloadResponse{PresignedGetURL: originURL}, nil
		},
	})

	c := single(t, mustRead(t, s, fileURI("abc")))
	if c.Text != "hello world" {
		t.Errorf("Text = %q, want %q", c.Text, "hello world")
	}
	if len(c.Blob) != 0 {
		t.Errorf("expected no Blob for textual content, got %d bytes", len(c.Blob))
	}
	if !strings.HasPrefix(c.MIMEType, "text/plain") {
		t.Errorf("MIMEType = %q, want text/plain…", c.MIMEType)
	}
}

// TestReadResource_BinaryContent: a binary file is returned as a Blob, not Text.
func TestReadResource_BinaryContent(t *testing.T) {
	payload := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x01, 0x02}
	originURL := serveContent(t, "image/png", payload)
	s := newTestServer(t, &stubClient{
		downloadFn: func(_ context.Context, _ string) (*client.DownloadResponse, error) {
			return &client.DownloadResponse{PresignedGetURL: originURL}, nil
		},
	})

	c := single(t, mustRead(t, s, fileURI("img")))
	if c.Text != "" {
		t.Errorf("expected no Text for binary content, got %q", c.Text)
	}
	if !bytes.Equal(c.Blob, payload) {
		t.Errorf("Blob = %v, want %v", c.Blob, payload)
	}
}

// TestReadResource_OverCapReturnsMetadata: a file past the inline cap returns
// metadata + a download URL instead of bytes.
func TestReadResource_OverCapReturnsMetadata(t *testing.T) {
	originURL := serveContent(t, "text/plain", []byte(strings.Repeat("A", 100)))

	old := maxResourceBytes
	maxResourceBytes = 8
	t.Cleanup(func() { maxResourceBytes = old })

	s := newTestServer(t, &stubClient{
		downloadFn: func(_ context.Context, _ string) (*client.DownloadResponse, error) {
			return &client.DownloadResponse{PresignedGetURL: originURL}, nil
		},
	})

	c := single(t, mustRead(t, s, fileURI("big")))
	if c.MIMEType != "application/json" {
		t.Errorf("MIMEType = %q, want application/json", c.MIMEType)
	}
	if len(c.Blob) != 0 {
		t.Errorf("expected no Blob for over-cap metadata, got %d bytes", len(c.Blob))
	}
	if !strings.Contains(c.Text, "downloadUrl") || !strings.Contains(c.Text, originURL) {
		t.Errorf("metadata text missing downloadUrl: %q", c.Text)
	}
}

// TestReadResource_RejectsMaliciousURI: malformed/foreign URIs are rejected
// before any backend call.
func TestReadResource_RejectsMaliciousURI(t *testing.T) {
	called := false
	s := newTestServer(t, &stubClient{
		downloadFn: func(_ context.Context, _ string) (*client.DownloadResponse, error) {
			called = true
			return &client.DownloadResponse{}, nil
		},
	})

	bad := []string{
		"pushkit://files/../etc/passwd",
		"pushkit://files/a/b",
		"pushkit://other/x",
		"file:///etc/passwd",
		"pushkit://files/",
		"pushkit://files/has space",
		"pushkit://files/" + strings.Repeat("z", 200),
	}
	for _, uri := range bad {
		if _, err := s.readFileResource(context.Background(), readReq(uri)); err == nil {
			t.Errorf("expected rejection for %q, got nil error", uri)
		}
	}
	if called {
		t.Error("GetDownloadURL must not be called for an invalid URI")
	}
}

// TestReadResource_TemplateFallbackResolvesUnlistedID: an ID that was never added
// as a concrete resource still resolves through the registered template.
func TestReadResource_TemplateFallbackResolvesUnlistedID(t *testing.T) {
	originURL := serveContent(t, "text/plain", []byte("template-content"))
	s := newTestServer(t, &stubClient{
		downloadFn: func(_ context.Context, _ string) (*client.DownloadResponse, error) {
			return &client.DownloadResponse{PresignedGetURL: originURL}, nil
		},
	})

	ctx := context.Background()
	cs := connectClient(t, ctx, s, nil)

	// "ghost-123" is not concretely registered (reconcile never ran) — only the
	// template exists, so this exercises the template fallback path.
	res, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: fileURI("ghost-123")})
	if err != nil {
		t.Fatalf("read via template: %v", err)
	}
	if len(res.Contents) != 1 || res.Contents[0].Text != "template-content" {
		t.Errorf("template read returned %+v", res.Contents)
	}
}

func mustRead(t *testing.T, s *Server, uri string) *mcp.ReadResourceResult {
	t.Helper()
	res, err := s.readFileResource(context.Background(), readReq(uri))
	if err != nil {
		t.Fatalf("readFileResource(%s): %v", uri, err)
	}
	return res
}
