package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/pushkit/cli/internal/client"
)

const pullDescription = "Download PushKit files to disk and return their local paths (never inline contents). Select with exactly one of: id, filename (newest exact match), newest, or all_unpulled. Files are written into CLAUDE_PROJECT_DIR (or ~/.pushkit/downloads); name collisions get a numeric suffix and the write is always confined to that directory."

// pullHTTPClient fetches presigned GET URLs. The timeout matches the backend's
// 900s presign TTL with headroom for large files.
var pullHTTPClient = &http.Client{Timeout: 15 * time.Minute}

type pullInput struct {
	ID          string `json:"id,omitempty" jsonschema:"pull the file with this exact ID"`
	Filename    string `json:"filename,omitempty" jsonschema:"pull the newest file with this exact filename"`
	Newest      bool   `json:"newest,omitempty" jsonschema:"pull the single most recently uploaded file"`
	AllUnpulled bool   `json:"all_unpulled,omitempty" jsonschema:"pull every file not pulled before from this backend"`
}

type pullFileResult struct {
	OK       bool   `json:"ok"`
	FileID   string `json:"file_id"`
	Filename string `json:"filename,omitempty"`
	Path     string `json:"path,omitempty"`
	Error    string `json:"error,omitempty"`
}

type pullOutput struct {
	TargetDir string           `json:"target_dir"`
	Pulled    int              `json:"pulled"`
	Failed    int              `json:"failed"`
	Results   []pullFileResult `json:"results"`
}

// target is a file selected for download. filename is empty for the id selector
// (it is then derived from the download response's Content-Disposition).
type target struct {
	id       string
	filename string
}

func (h *handlers) pull(ctx context.Context, _ *mcp.CallToolRequest, in pullInput) (*mcp.CallToolResult, any, error) {
	if err := h.requireKey(); err != nil {
		return nil, nil, err
	}

	targets, err := h.resolveTargets(ctx, in)
	if err != nil {
		return nil, nil, err
	}
	if len(targets) == 0 {
		return nil, nil, fmt.Errorf("no files matched the selector")
	}

	targetDir, err := h.pullTargetDir()
	if err != nil {
		return nil, nil, err
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create target dir: %w", err)
	}
	// os.Root confines every write below targetDir at the OS level.
	root, err := os.OpenRoot(targetDir)
	if err != nil {
		return nil, nil, fmt.Errorf("open target dir: %w", err)
	}
	defer root.Close()

	out := pullOutput{TargetDir: targetDir, Results: make([]pullFileResult, 0, len(targets))}
	var pulledIDs []string
	for _, tgt := range targets {
		r := h.pullOne(ctx, root, targetDir, tgt)
		out.Results = append(out.Results, r)
		if r.OK {
			out.Pulled++
			pulledIDs = append(pulledIDs, r.FileID)
		} else {
			out.Failed++
		}
	}

	if h.state != nil && len(pulledIDs) > 0 {
		if err := h.state.MarkPulled(pulledIDs...); err != nil {
			fmt.Fprintf(os.Stderr, "pushkit mcp: failed to persist pulled state: %v\n", err)
		}
	}

	data, err := json.Marshal(out)
	if err != nil {
		return nil, nil, fmt.Errorf("encode pull result: %w", err)
	}
	return textResult(string(data)), nil, nil
}

// resolveTargets turns the selector into the list of files to download.
func (h *handlers) resolveTargets(ctx context.Context, in pullInput) ([]target, error) {
	n := 0
	for _, set := range []bool{in.ID != "", in.Filename != "", in.Newest, in.AllUnpulled} {
		if set {
			n++
		}
	}
	if n == 0 {
		return nil, fmt.Errorf("specify a selector: id, filename, newest, or all_unpulled")
	}
	if n > 1 {
		return nil, fmt.Errorf("specify exactly one selector, not several")
	}

	switch {
	case in.ID != "":
		return []target{{id: in.ID}}, nil

	case in.Filename != "":
		resp, err := h.c.ListFiles(ctx, "", maxListLimit, in.Filename, "", "")
		if err != nil {
			return nil, fmt.Errorf("list files: %w", err)
		}
		for _, f := range resp.Items { // backend lists newest-first
			if f.OriginalFilename == in.Filename {
				return []target{{id: f.ID, filename: f.OriginalFilename}}, nil
			}
		}
		return nil, fmt.Errorf("no file named %q found", in.Filename)

	case in.Newest:
		resp, err := h.c.ListFiles(ctx, "", 1, "", "", "")
		if err != nil {
			return nil, fmt.Errorf("list files: %w", err)
		}
		if len(resp.Items) == 0 {
			return nil, fmt.Errorf("no files to pull")
		}
		f := resp.Items[0]
		return []target{{id: f.ID, filename: f.OriginalFilename}}, nil

	default: // AllUnpulled
		all, err := h.listAll(ctx)
		if err != nil {
			return nil, err
		}
		var out []target
		for _, f := range all {
			if h.state == nil || !h.state.IsPulled(f.ID) {
				out = append(out, target{id: f.ID, filename: f.OriginalFilename})
			}
		}
		return out, nil
	}
}

// listAll paginates the full file list (bounded by maxMatches as a safety stop).
func (h *handlers) listAll(ctx context.Context) ([]client.FileResponse, error) {
	var all []client.FileResponse
	cursor := ""
	for {
		resp, err := h.c.ListFiles(ctx, cursor, maxListLimit, "", "", "")
		if err != nil {
			return nil, fmt.Errorf("list files: %w", err)
		}
		all = append(all, resp.Items...)
		if resp.NextCursor == nil || *resp.NextCursor == "" || len(all) > maxMatches {
			return all, nil
		}
		cursor = *resp.NextCursor
	}
}

// pullOne downloads one file and writes it confined to targetDir, returning the
// local path. Any failure is captured in the result, not propagated as a tool
// error, so a batch (all_unpulled) reports per-file outcomes.
func (h *handlers) pullOne(ctx context.Context, root *os.Root, targetDir string, tgt target) pullFileResult {
	r := pullFileResult{FileID: tgt.id, Filename: tgt.filename}

	dl, err := h.c.GetDownloadURL(ctx, tgt.id)
	if err != nil {
		r.Error = fmt.Sprintf("download url: %v", err)
		return r
	}
	resp, err := fetch(ctx, dl.PresignedGetURL)
	if err != nil {
		r.Error = fmt.Sprintf("fetch: %v", err)
		return r
	}
	defer resp.Body.Close()

	filename := tgt.filename
	if filename == "" {
		filename = filenameFromResponse(resp, tgt.id)
	}
	r.Filename = filename

	path, err := writeConfined(root, targetDir, filename, resp.Body)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	r.Path = path
	r.OK = true
	return r
}

// writeConfined writes body to a uniquely-named file inside targetDir. The
// server-supplied filename is treated as hostile: safeName rejects traversal,
// absolute paths, separators, and null bytes outright, and os.Root is the OS-level
// backstop that confines the actual write. Collisions get a numeric suffix via an
// O_EXCL create loop (no stat-then-create race, and never follows/clobbers an
// existing entry such as a symlink).
func writeConfined(root *os.Root, targetDir, filename string, body io.Reader) (string, error) {
	safe, err := safeName(filename)
	if err != nil {
		return "", err
	}
	for n := 0; ; n++ {
		name := suffixed(safe, n)
		f, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue // collision (or an existing symlink) → next suffix
		}
		if err != nil {
			return "", fmt.Errorf("create %q in target dir: %w", name, err) // os.Root rejected an escape
		}
		if _, err := io.Copy(f, body); err != nil {
			f.Close()
			_ = root.Remove(name)
			return "", fmt.Errorf("write %q: %w", name, err)
		}
		if err := f.Close(); err != nil {
			return "", fmt.Errorf("close %q: %w", name, err)
		}
		return filepath.Join(targetDir, name), nil
	}
}

// safeName rejects a server-supplied filename that is not a single, benign path
// element. This is the explicit traversal-rejection gate; os.Root backs it up.
func safeName(filename string) (string, error) {
	switch {
	case strings.ContainsRune(filename, 0):
		return "", fmt.Errorf("server filename contains a null byte; rejected")
	case filename == "" || filename == "." || filename == "..":
		return "", fmt.Errorf("server filename %q is not a valid name; rejected", filename)
	case filepath.IsAbs(filename):
		return "", fmt.Errorf("server filename %q is absolute; rejected", filename)
	case strings.ContainsAny(filename, `/\`):
		return "", fmt.Errorf("server filename %q contains a path separator; rejected", filename)
	case strings.Contains(filename, ".."):
		return "", fmt.Errorf("server filename %q contains a parent reference; rejected", filename)
	}
	return filename, nil
}

// suffixed returns name for n==0, then name-2.ext, name-3.ext, … preserving the
// extension.
func suffixed(name string, n int) string {
	if n == 0 {
		return name
	}
	ext := filepath.Ext(name)
	return fmt.Sprintf("%s-%d%s", strings.TrimSuffix(name, ext), n+1, ext)
}

// pullTargetDir resolves where files are written: CLAUDE_PROJECT_DIR when Claude
// Code injects it, else ~/.pushkit/downloads.
func (h *handlers) pullTargetDir() (string, error) {
	if d := os.Getenv("CLAUDE_PROJECT_DIR"); d != "" {
		return d, nil
	}
	if h.stateDir != "" {
		return filepath.Join(h.stateDir, "downloads"), nil
	}
	home, err := pushkitHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "downloads"), nil
}

func fetch(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := pullHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return resp, nil
}

// filenameFromResponse extracts a filename from the Content-Disposition header,
// falling back to the file ID when absent.
func filenameFromResponse(resp *http.Response, fallback string) string {
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			if fn := params["filename"]; fn != "" {
				return fn
			}
		}
	}
	return fallback
}
