package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/pushkit/cli/internal/client"
)

// Files are exposed to the MCP client as read-only resources under the
// pushkit://files/{id} scheme so an agent can @-mention them. The URI's {id} is
// client-controlled and flows into a backend path, so it is validated before any
// backend call (see parseFileURI / validFileID).
const (
	resourceScheme = "pushkit"
	resourceHost   = "files"
)

// maxResourceBytes caps how many bytes a single resource read embeds inline. Over
// the cap the read returns the file's metadata plus its download URL instead of
// the bytes, so an @-mention of a large file never blows the MCP output budget.
// It is a var (not a const) so tests can shrink it.
var maxResourceBytes int64 = 10 << 20 // 10 MiB

// fileURI builds the canonical resource URI for a file ID.
func fileURI(id string) string {
	return resourceScheme + "://" + resourceHost + "/" + id
}

// parseFileURI extracts the file ID from a pushkit://files/{id} URI. The scheme,
// host, and ID are all validated; the ID must be a single benign token (UUID
// shape) — anything with a separator, parent reference, control byte, or the
// wrong scheme is rejected so a malicious URI cannot reach the backend. ok is
// false on any malformed input.
func parseFileURI(raw string) (id string, ok bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != resourceScheme || u.Host != resourceHost {
		return "", false
	}
	id = strings.TrimPrefix(u.Path, "/")
	if !validFileID(id) {
		return "", false
	}
	return id, true
}

// validFileID accepts only the UUID-shaped token the backend issues: ASCII
// letters, digits, '-' and '_'. This rejects path separators, '.'/'..', spaces,
// and control bytes outright — the resource read's first line of defense.
func validFileID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

// registerResourceTemplate registers the pushkit://files/{id} template once at
// startup, sharing the same read handler as the concrete resources. It advertises
// the resources capability immediately and serves direct-URI reads for files that
// are not currently in the concrete list.
func (s *Server) registerResourceTemplate() {
	s.mcp.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "PushKit file",
		Title:       "PushKit file",
		Description: "A file stored in PushKit, addressable by ID as pushkit://files/{id}.",
		URITemplate: resourceScheme + "://" + resourceHost + "/{id}",
	}, s.readFileResource)
}

// addFileResource registers one file as a concrete, @-mentionable resource. The
// SDK fires notifications/resources/list_changed automatically (debounced).
func (s *Server) addFileResource(f client.FileResponse) {
	r := &mcp.Resource{
		Name:        f.OriginalFilename,
		Title:       f.OriginalFilename,
		Description: resourceDescription(f),
		MIMEType:    f.ContentType,
		URI:         fileURI(f.ID),
	}
	if f.SizeBytes != nil {
		r.Size = *f.SizeBytes
	}
	s.mcp.AddResource(r, s.readFileResource)
}

// resourceDescription renders the compact "size · created" subtitle shown beside
// a file in the @-mention picker.
func resourceDescription(f client.FileResponse) string {
	var parts []string
	if f.SizeBytes != nil {
		parts = append(parts, humanSize(*f.SizeBytes))
	}
	if f.CreatedAt != "" {
		parts = append(parts, f.CreatedAt)
	}
	return strings.Join(parts, " · ")
}

// reconcileResources re-lists the backend and brings the concrete resource set
// into sync: files that appeared are registered, files that are gone are removed,
// and each change makes the SDK emit resources/list_changed. This is the single
// refresh primitive — run on (re)connect and on each non-echo file.uploaded — and
// it needs no missed-event replay because it always reflects the current list.
func (s *Server) reconcileResources(ctx context.Context) error {
	files, err := s.h.listAll(ctx)
	if err != nil {
		return err
	}
	desired := make(map[string]struct{}, len(files))
	for _, f := range files {
		desired[f.ID] = struct{}{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, f := range files {
		if _, ok := s.registered[f.ID]; ok {
			continue
		}
		s.addFileResource(f)
		s.registered[f.ID] = struct{}{}
	}

	var gone []string
	for id := range s.registered {
		if _, ok := desired[id]; !ok {
			gone = append(gone, fileURI(id))
			delete(s.registered, id)
		}
	}
	if len(gone) > 0 {
		s.mcp.RemoveResources(gone...)
	}
	return nil
}

// readFileResource resolves a pushkit://files/{id} URI to the file's contents. It
// validates the URI, fetches the file through its presigned URL, and returns the
// bytes inline (text for textual UTF-8 content, otherwise a blob). Files larger
// than maxResourceBytes are returned as metadata plus the download URL rather than
// inline bytes. Shared by both the concrete resources and the template.
func (s *Server) readFileResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	uri := req.Params.URI
	id, ok := parseFileURI(uri)
	if !ok {
		return nil, mcp.ResourceNotFoundError(uri)
	}

	dl, err := s.h.c.GetDownloadURL(ctx, id)
	if err != nil {
		return nil, mcp.ResourceNotFoundError(uri)
	}

	resp, err := fetch(ctx, dl.PresignedGetURL)
	if err != nil {
		return nil, fmt.Errorf("fetch resource: %w", err)
	}
	defer resp.Body.Close()

	// Read one byte past the cap so an over-cap file is detected without buffering
	// the whole thing.
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResourceBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read resource: %w", err)
	}
	contentType := resp.Header.Get("Content-Type")

	if int64(len(data)) > maxResourceBytes {
		return oversizeResult(uri, id, contentType, dl.PresignedGetURL), nil
	}
	if isTextual(contentType) && utf8.Valid(data) {
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
			URI:      uri,
			MIMEType: contentType,
			Text:     string(data),
		}}}, nil
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
		URI:      uri,
		MIMEType: contentType,
		Blob:     data,
	}}}, nil
}

// oversizeResult is returned when a file exceeds the inline cap: the agent gets
// the file's identity and a download URL instead of the bytes.
func oversizeResult(uri, id, contentType, downloadURL string) *mcp.ReadResourceResult {
	payload := map[string]any{
		"id":          id,
		"contentType": contentType,
		"downloadUrl": downloadURL,
		"note":        fmt.Sprintf("file exceeds the %d-byte inline cap; fetch it via downloadUrl", maxResourceBytes),
	}
	data, _ := json.Marshal(payload)
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
		URI:      uri,
		MIMEType: "application/json",
		Text:     string(data),
	}}}
}

// isTextual reports whether a content type should be returned as inline text
// rather than a base64 blob.
func isTextual(contentType string) bool {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	if strings.HasPrefix(ct, "text/") {
		return true
	}
	switch ct {
	case "application/json", "application/xml", "application/yaml",
		"application/x-yaml", "application/javascript", "application/x-ndjson",
		"image/svg+xml":
		return true
	}
	return strings.HasSuffix(ct, "+json") || strings.HasSuffix(ct, "+xml")
}

// humanSize renders a byte count compactly (e.g. "1.5 MB").
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
