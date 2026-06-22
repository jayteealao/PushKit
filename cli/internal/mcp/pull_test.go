package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/pushkit/cli/internal/client"
)

// fileServer serves fixed bytes for any path and echoes a Content-Disposition
// from the "cd" query param when present.
func fileServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cd := r.URL.Query().Get("cd"); cd != "" {
			w.Header().Set("Content-Disposition", cd)
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func parsePull(t *testing.T, res *mcp.CallToolResult) pullOutput {
	t.Helper()
	var out pullOutput
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("decode pull output: %v", err)
	}
	return out
}

// TestPullRejectsTraversal is the highest-priority security gate (AC#3): a
// server-supplied filename that tries to escape the target dir is rejected and
// nothing is written outside it.
func TestPullRejectsTraversal(t *testing.T) {
	srv := fileServer(t, "PAYLOAD")

	malicious := []string{
		"../escape.txt",
		"../../etc/passwd",
		"/abs.txt",
		`..\win.txt`,
		"..",
		"sub/inner.txt",
		"with\x00null.txt",
	}

	for _, name := range malicious {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			inside := filepath.Join(root, "inside")
			outside := filepath.Join(root, "outside")
			if err := os.MkdirAll(inside, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(outside, 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("CLAUDE_PROJECT_DIR", inside)

			stub := &stubClient{
				listFn: func(_ context.Context, _ string, _ int, _, _, _ string) (*client.FileListResponse, error) {
					return &client.FileListResponse{Items: []client.FileResponse{
						{ID: "evil", OriginalFilename: name},
					}}, nil
				},
				downloadFn: func(_ context.Context, _ string) (*client.DownloadResponse, error) {
					return &client.DownloadResponse{PresignedGetURL: srv.URL + "/dl"}, nil
				},
			}
			h := &handlers{c: stub, apiKey: "k", state: LoadState(t.TempDir(), "http://b")}

			res, _, err := h.pull(context.Background(), nil, pullInput{Newest: true})
			if err != nil {
				t.Fatalf("pull returned a protocol error: %v", err)
			}
			out := parsePull(t, res)
			if out.Pulled != 0 || out.Failed != 1 {
				t.Fatalf("expected the malicious name to be rejected, got %+v", out)
			}
			if !strings.Contains(strings.ToLower(out.Results[0].Error), "reject") {
				t.Errorf("error should say the name was rejected, got %q", out.Results[0].Error)
			}

			// Nothing escaped to the sibling dir, and the inside dir got no payload file.
			if entries, _ := os.ReadDir(outside); len(entries) != 0 {
				t.Errorf("a file escaped to the outside dir: %v", entries)
			}
			_ = filepath.WalkDir(inside, func(p string, d os.DirEntry, _ error) error {
				if !d.IsDir() {
					if data, _ := os.ReadFile(p); string(data) == "PAYLOAD" {
						t.Errorf("payload was written inside as %s for malicious name %q", p, name)
					}
				}
				return nil
			})
		})
	}
}

// TestPullCollisionSuffix covers AC#4: a second file with the same name is
// written as name-2.ext, never overwriting the first.
func TestPullCollisionSuffix(t *testing.T) {
	srv := fileServer(t, "SECOND")
	dir := t.TempDir()
	t.Setenv("CLAUDE_PROJECT_DIR", dir)

	// Pre-existing file the pull must not overwrite.
	if err := os.WriteFile(filepath.Join(dir, "report.pdf"), []byte("FIRST"), 0o644); err != nil {
		t.Fatal(err)
	}

	stub := &stubClient{
		listFn: func(_ context.Context, _ string, _ int, _, _, _ string) (*client.FileListResponse, error) {
			return &client.FileListResponse{Items: []client.FileResponse{
				{ID: "id1", OriginalFilename: "report.pdf"},
			}}, nil
		},
		downloadFn: func(_ context.Context, _ string) (*client.DownloadResponse, error) {
			return &client.DownloadResponse{PresignedGetURL: srv.URL + "/dl"}, nil
		},
	}
	h := &handlers{c: stub, apiKey: "k", state: LoadState(t.TempDir(), "http://b")}

	res, _, err := h.pull(context.Background(), nil, pullInput{Newest: true})
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	out := parsePull(t, res)
	if out.Pulled != 1 {
		t.Fatalf("expected 1 pulled, got %+v", out)
	}
	if filepath.Base(out.Results[0].Path) != "report-2.pdf" {
		t.Errorf("collision path = %q, want …/report-2.pdf", out.Results[0].Path)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "report.pdf")); string(data) != "FIRST" {
		t.Errorf("original report.pdf was overwritten (got %q)", data)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "report-2.pdf")); string(data) != "SECOND" {
		t.Errorf("report-2.pdf contents = %q, want SECOND", data)
	}
}

// TestPullByIDUsesContentDisposition verifies the id selector derives the local
// filename from the response and marks the file pulled.
func TestPullByIDUsesContentDisposition(t *testing.T) {
	srv := fileServer(t, "HELLO")
	dir := t.TempDir()
	t.Setenv("CLAUDE_PROJECT_DIR", dir)

	stub := &stubClient{
		downloadFn: func(_ context.Context, id string) (*client.DownloadResponse, error) {
			// Encode the whole Content-Disposition as one query value so the
			// semicolon survives (Go's query parser drops a bare ';').
			q := url.Values{"cd": {`attachment; filename="hello.txt"`}}.Encode()
			return &client.DownloadResponse{PresignedGetURL: srv.URL + "/dl?" + q}, nil
		},
	}
	state := LoadState(t.TempDir(), "http://b")
	h := &handlers{c: stub, apiKey: "k", state: state}

	res, _, err := h.pull(context.Background(), nil, pullInput{ID: "id-7"})
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	out := parsePull(t, res)
	if out.Pulled != 1 || filepath.Base(out.Results[0].Path) != "hello.txt" {
		t.Fatalf("expected hello.txt pulled, got %+v", out)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "hello.txt")); string(data) != "HELLO" {
		t.Errorf("downloaded contents = %q, want HELLO", data)
	}
	if !state.IsPulled("id-7") {
		t.Error("id-7 should be marked pulled after a successful pull")
	}
}

// TestPullAllUnpulledSkipsPulled covers the sync-style selector: already-pulled
// IDs are skipped.
func TestPullAllUnpulledSkipsPulled(t *testing.T) {
	srv := fileServer(t, "DATA")
	dir := t.TempDir()
	t.Setenv("CLAUDE_PROJECT_DIR", dir)

	stub := &stubClient{
		listFn: func(_ context.Context, _ string, _ int, _, _, _ string) (*client.FileListResponse, error) {
			return &client.FileListResponse{Items: []client.FileResponse{
				{ID: "id1", OriginalFilename: "a.txt"},
				{ID: "id2", OriginalFilename: "b.txt"},
			}}, nil
		},
		downloadFn: func(_ context.Context, _ string) (*client.DownloadResponse, error) {
			return &client.DownloadResponse{PresignedGetURL: srv.URL + "/dl"}, nil
		},
	}
	state := LoadState(t.TempDir(), "http://b")
	if err := state.MarkPulled("id1"); err != nil { // id1 already pulled
		t.Fatal(err)
	}
	h := &handlers{c: stub, apiKey: "k", state: state}

	res, _, err := h.pull(context.Background(), nil, pullInput{AllUnpulled: true})
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	out := parsePull(t, res)
	if out.Pulled != 1 || out.Results[0].FileID != "id2" {
		t.Fatalf("expected only id2 pulled, got %+v", out)
	}
	if !state.IsPulled("id2") {
		t.Error("id2 should be marked pulled")
	}
}

func TestPullRequiresSelector(t *testing.T) {
	h := &handlers{c: &stubClient{}, apiKey: "k"}
	if _, _, err := h.pull(context.Background(), nil, pullInput{}); err == nil {
		t.Error("expected an error when no selector is given")
	}
}
