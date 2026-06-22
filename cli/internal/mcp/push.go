package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/pushkit/cli/internal/client"
)

const pushDescription = "Upload one or more local files to PushKit so they appear on the user's other devices. Accepts explicit paths, directories (uploaded recursively), and globs (** and {a,b}); relative paths resolve against the project directory. Best-effort: each file is uploaded independently and the result lists per-file success or failure."

type pushInput struct {
	Paths []string `json:"paths" jsonschema:"file paths, directories, or globs to upload; relative paths resolve against the project directory"`
}

type pushFileResult struct {
	OK       bool   `json:"ok"`
	Path     string `json:"path"`
	Filename string `json:"filename,omitempty"`
	FileID   string `json:"file_id,omitempty"`
	Error    string `json:"error,omitempty"`
}

type pushOutput struct {
	Matched   int              `json:"matched"`
	Succeeded int              `json:"succeeded"`
	Failed    int              `json:"failed"`
	Results   []pushFileResult `json:"results"`
}

func (h *handlers) push(ctx context.Context, _ *mcp.CallToolRequest, in pushInput) (*mcp.CallToolResult, any, error) {
	if err := h.requireKey(); err != nil {
		return nil, nil, err
	}
	if len(in.Paths) == 0 {
		return nil, nil, fmt.Errorf("no paths given")
	}

	files, err := expand(projectDir(), in.Paths)
	if err != nil {
		return nil, nil, err
	}
	if len(files) == 0 {
		return nil, nil, fmt.Errorf("no files matched %v", in.Paths)
	}

	out := pushOutput{Matched: len(files), Results: make([]pushFileResult, 0, len(files))}
	for _, path := range files {
		r := h.pushOne(ctx, path)
		out.Results = append(out.Results, r)
		if r.OK {
			out.Succeeded++
		} else {
			out.Failed++
		}
	}

	data, err := json.Marshal(out)
	if err != nil {
		return nil, nil, fmt.Errorf("encode push result: %w", err)
	}
	return textResult(string(data)), nil, nil
}

// pushOne uploads a single file via init -> presigned PUT -> complete. It is
// best-effort: on any failure after init, it tries to delete the orphaned
// INITIATED row the init created.
func (h *handlers) pushOne(ctx context.Context, path string) pushFileResult {
	r := pushFileResult{Path: path, Filename: filepath.Base(path)}

	info, err := os.Stat(path)
	if err != nil {
		r.Error = fmt.Sprintf("stat: %v", err)
		return r
	}
	f, err := os.Open(path)
	if err != nil {
		r.Error = fmt.Sprintf("open: %v", err)
		return r
	}
	defer f.Close()

	contentType := detectContentType(r.Filename)
	sz := info.Size()
	initResp, err := h.c.InitUpload(ctx, &client.UploadInitRequest{
		Filename:    r.Filename,
		ContentType: contentType,
		SizeBytes:   &sz,
	})
	if err != nil {
		r.Error = fmt.Sprintf("init: %v", err)
		return r
	}
	r.FileID = initResp.FileID

	// The presigned PUT must echo the Content-Type baked into the signature.
	putContentType := contentType
	if ct := initResp.RequiredHeaders["Content-Type"]; ct != "" {
		putContentType = ct
	}

	if err := h.c.PutToPresignedURL(ctx, initResp.PresignedPutURL, f, putContentType, info.Size()); err != nil {
		r.Error = fmt.Sprintf("upload: %v", err)
		h.cleanupOrphan(ctx, initResp.FileID)
		return r
	}
	if _, err := h.c.CompleteUpload(ctx, &client.UploadCompleteRequest{
		FileID:    initResp.FileID,
		SizeBytes: &sz,
	}); err != nil {
		r.Error = fmt.Sprintf("complete: %v", err)
		h.cleanupOrphan(ctx, initResp.FileID)
		return r
	}

	r.OK = true
	if h.state != nil {
		h.state.RecordPushed(initResp.FileID) // self-echo de-dup (consumed by auto-refresh)
	}
	return r
}

// cleanupOrphan best-effort removes the INITIATED row a failed push created.
// DELETE accepts any status, so this works even though the upload never
// completed. Diagnostics go to stderr — stdout is the JSON-RPC channel.
func (h *handlers) cleanupOrphan(ctx context.Context, fileID string) {
	if err := h.c.Delete(ctx, fileID); err != nil {
		fmt.Fprintf(os.Stderr, "pushkit mcp: orphan cleanup failed for %s: %v\n", fileID, err)
	}
}

// projectDir is the base for resolving relative push paths: CLAUDE_PROJECT_DIR
// when Claude Code injects it, else the current working directory.
func projectDir() string {
	if d := os.Getenv("CLAUDE_PROJECT_DIR"); d != "" {
		return d
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

// detectContentType maps a filename extension to a MIME type, defaulting to
// application/octet-stream.
func detectContentType(filename string) string {
	if ext := filepath.Ext(filename); ext != "" {
		if ct := mime.TypeByExtension(ext); ct != "" {
			return ct
		}
	}
	return "application/octet-stream"
}
