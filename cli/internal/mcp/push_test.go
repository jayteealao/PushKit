package mcp

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pushkit/cli/internal/client"
)

func writeFile(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("contents of "+name), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPushBestEffortWithOrphanCleanup(t *testing.T) {
	dir := t.TempDir()
	f1 := writeFile(t, dir, "f1.txt")
	f2 := writeFile(t, dir, "f2.txt")

	// f2's complete fails; f1 succeeds. The stub's default InitUpload sets
	// FileID = "file-" + filename.
	stub := &stubClient{
		completeFn: func(_ context.Context, req *client.UploadCompleteRequest) (*client.FileResponse, error) {
			if req.FileID == "file-f2.txt" {
				return nil, fmt.Errorf("simulated complete failure")
			}
			return &client.FileResponse{ID: req.FileID, Status: "UPLOADED"}, nil
		},
	}
	state := LoadState(t.TempDir(), "http://localhost:8080")
	h := &handlers{c: stub, apiKey: "k", state: state}

	res, _, err := h.push(context.Background(), nil, pushInput{Paths: []string{f1, f2}})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	text := resultText(t, res)

	// One success, one failure.
	for _, want := range []string{`"matched":2`, `"succeeded":1`, `"failed":1`, "simulated complete failure"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in result\ngot: %s", want, text)
		}
	}

	// The failed file's orphaned INITIATED row was cleaned up.
	if len(stub.deleted) != 1 || stub.deleted[0] != "file-f2.txt" {
		t.Errorf("expected orphan cleanup Delete(file-f2.txt), got deleted=%v", stub.deleted)
	}

	// The succeeded file was recorded for self-echo de-dup; the failed one was not.
	if !state.WasRecentlyPushed("file-f1.txt", time.Minute) {
		t.Error("succeeded push should be recorded in state")
	}
	if state.WasRecentlyPushed("file-f2.txt", time.Minute) {
		t.Error("failed push must not be recorded in state")
	}
}

func TestPushNoMatch(t *testing.T) {
	h := &handlers{c: &stubClient{}, apiKey: "k"}
	_, _, err := h.push(context.Background(), nil, pushInput{Paths: []string{filepath.Join(t.TempDir(), "ghost-*.bin")}})
	if err == nil {
		t.Error("expected an error when nothing matches")
	}
}

func TestPushEchoesSignedContentType(t *testing.T) {
	f := writeFile(t, t.TempDir(), "doc.txt")

	var putCT string
	stub := &stubClient{
		initFn: func(_ context.Context, _ *client.UploadInitRequest) (*client.UploadInitResponse, error) {
			// Backend returns the Content-Type baked into the presign signature.
			return &client.UploadInitResponse{
				FileID:          "fid",
				PresignedPutURL: "http://put.invalid/o",
				RequiredHeaders: map[string]string{"Content-Type": "application/x-signed"},
			}, nil
		},
		putFn: func(_ context.Context, _ string, body io.Reader, ct string, _ int64) error {
			putCT = ct
			_, _ = io.Copy(io.Discard, body)
			return nil
		},
	}
	h := &handlers{c: stub, apiKey: "k"}

	if _, _, err := h.push(context.Background(), nil, pushInput{Paths: []string{f}}); err != nil {
		t.Fatalf("push: %v", err)
	}
	if putCT != "application/x-signed" {
		t.Errorf("PUT Content-Type = %q, want the signed value from requiredHeaders", putCT)
	}
}
