// Package mcp implements the `pushkit mcp` stdio MCP server: four file tools
// (pushkit_push / pushkit_pull / pushkit_list / pushkit_delete) that an MCP
// client such as Claude Code can call to move files to and from a PushKit
// backend. It is a thin layer over the existing REST client; the only new
// behavior is glob-expanded push, traversal-safe pull (os.Root), a confirm-gated
// delete, and a client-side pulled-ID store.
//
// stdout is reserved for the JSON-RPC protocol — this package never writes
// diagnostics to stdout. Tool failures are returned as errors, which the SDK
// packs into the tool result with IsError set (a tool-level error the model can
// read), not as protocol errors.
package mcp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/pushkit/cli/internal/client"
)

// apiClient is the subset of *client.Client the tools use. Defining it here lets
// tests inject an in-memory stub; *client.Client satisfies it.
type apiClient interface {
	InitUpload(ctx context.Context, req *client.UploadInitRequest) (*client.UploadInitResponse, error)
	CompleteUpload(ctx context.Context, req *client.UploadCompleteRequest) (*client.FileResponse, error)
	PutToPresignedURL(ctx context.Context, presignedURL string, body io.Reader, contentType string, size int64) error
	ListFiles(ctx context.Context, cursor string, limit int, search, sort, order string) (*client.FileListResponse, error)
	GetDownloadURL(ctx context.Context, fileID string) (*client.DownloadResponse, error)
	Delete(ctx context.Context, fileID string) error
}

// handlers holds the dependencies shared by the four tool handlers.
type handlers struct {
	c        apiClient
	apiKey   string // used only by the start-and-error guard; never logged
	state    *State
	stateDir string // ~/.pushkit — the pull download fallback base
}

// Options configures a Server.
type Options struct {
	Client   apiClient
	APIKey   string // for the pre-call auth guard only
	State    *State
	StateDir string
	BaseURL  string // backend base URL; enables the live event subscription when set with APIKey
	Version  string
}

// Server wraps the MCP server and its stdio run loop, plus the live-refresh
// machinery: each backend file is exposed as a read-only pushkit://files/{id}
// resource, and a background subscriber to GET /v1/events keeps that set in sync.
type Server struct {
	mcp     *mcp.Server
	h       *handlers
	baseURL string

	mu         sync.Mutex
	registered map[string]struct{} // file IDs currently exposed as concrete resources

	// reconnect parameters for the SSE subscriber — per-instance so tests can
	// override them without mutating package-level state (race-safe).
	sseClient        *http.Client
	reconnectBase    time.Duration
	reconnectMax     time.Duration
}

// New constructs the pushkit MCP server with the four file tools registered. It
// never fails on missing credentials: the server starts regardless, and each
// tool call surfaces a clear auth/connection error (the "start-and-error"
// behavior).
func New(opts Options) *Server {
	version := opts.Version
	if version == "" {
		version = "dev"
	}
	h := &handlers{
		c:        opts.Client,
		apiKey:   opts.APIKey,
		state:    opts.State,
		stateDir: opts.StateDir,
	}

	s := mcp.NewServer(&mcp.Implementation{Name: "pushkit", Version: version}, nil)
	mcp.AddTool(s, &mcp.Tool{Name: "pushkit_push", Description: pushDescription}, h.push)
	mcp.AddTool(s, &mcp.Tool{Name: "pushkit_pull", Description: pullDescription}, h.pull)
	mcp.AddTool(s, &mcp.Tool{Name: "pushkit_list", Description: listDescription}, h.list)
	mcp.AddTool(s, &mcp.Tool{Name: "pushkit_delete", Description: deleteDescription}, h.delete)

	srv := &Server{
		mcp:           s,
		h:             h,
		baseURL:       opts.BaseURL,
		registered:    map[string]struct{}{},
		sseClient:     newSSEClient(),
		reconnectBase: 1 * time.Second,
		reconnectMax:  60 * time.Second,
	}
	// Register the resource template up front: it advertises the resources
	// capability (with listChanged) from startup and resolves direct-URI reads for
	// files not currently in the concrete list. Concrete per-file resources are
	// added by the event subscriber's reconcile.
	srv.registerResourceTemplate()
	return srv
}

// NewForClient builds a Server wired to a real backend client, loading the
// pulled-ID state from ~/.pushkit (keyed by baseURL) and using that directory as
// the pull download fallback. It never fails: if the home directory cannot be
// resolved, state persistence is disabled and a note is written to stderr.
func NewForClient(c apiClient, apiKey, baseURL, version string) *Server {
	dir, err := pushkitHome()
	if err != nil {
		fmt.Fprintf(os.Stderr, "pushkit mcp: could not resolve home dir, state disabled: %v\n", err)
		dir = ""
	}
	return New(Options{
		Client:   c,
		APIKey:   apiKey,
		State:    LoadState(dir, baseURL),
		StateDir: dir,
		BaseURL:  baseURL,
		Version:  version,
	})
}

// Run serves the MCP protocol over the given transport (stdio in production)
// until the client disconnects or ctx is cancelled. When credentials and a base
// URL are configured it also starts a best-effort background subscriber that
// keeps the file resources live; the subscriber is bound to Run's lifetime and
// never blocks the protocol loop or fails server startup.
func (s *Server) Run(ctx context.Context, t mcp.Transport) error {
	if s.subscriberEnabled() {
		subCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		go s.runSubscriber(subCtx)
	}
	return s.mcp.Run(ctx, t)
}

// subscriberEnabled reports whether the live event subscription can run: it needs
// both an API key and a base URL. Without them the server still starts and serves
// tools and the resource template (start-and-error), just without live refresh.
func (s *Server) subscriberEnabled() bool {
	return s.h != nil && s.h.apiKey != "" && s.baseURL != ""
}

// requireKey returns a clear tool error when no API key is configured. Tool
// handlers call it first so a server started without credentials still responds
// with an actionable message instead of crashing or returning a raw 401.
func (h *handlers) requireKey() error {
	if h.apiKey == "" {
		return fmt.Errorf("no API key configured: set PUSHKIT_API_KEY in the environment or run `pushkit config set --api-key=<key>`")
	}
	return nil
}

// textResult builds a successful tool result carrying a single text block. The
// text is the machine-and-human-readable payload (compact JSON or a path); the
// tools never return file bytes inline.
func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}
