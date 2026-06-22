package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const listDescription = "List your uploaded PushKit files (compact: id, filename, size, created_at), newest first and paginated. Pass the returned next_cursor to fetch the next page; use query to filter by filename substring."

const (
	defaultListLimit = 50
	maxListLimit     = 100
)

type listInput struct {
	Cursor string `json:"cursor,omitempty" jsonschema:"opaque pagination cursor from a previous call's next_cursor"`
	Limit  int    `json:"limit,omitempty" jsonschema:"max entries to return, 1-100 (default 50)"`
	Query  string `json:"query,omitempty" jsonschema:"case-insensitive filename substring filter"`
}

// compactFile is the token-frugal projection returned to the client: only the
// four fields the agent needs. Status, content type, and S3 keys are omitted to
// stay well under the MCP output cap.
type compactFile struct {
	ID        string `json:"id"`
	Filename  string `json:"filename"`
	Size      *int64 `json:"size"`
	CreatedAt string `json:"created_at"`
}

type listOutput struct {
	Files      []compactFile `json:"files"`
	Count      int           `json:"count"`
	NextCursor string        `json:"next_cursor,omitempty"`
}

func (h *handlers) list(ctx context.Context, _ *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
	if err := h.requireKey(); err != nil {
		return nil, nil, err
	}

	limit := in.Limit
	switch {
	case limit <= 0:
		limit = defaultListLimit
	case limit > maxListLimit:
		limit = maxListLimit
	}

	resp, err := h.c.ListFiles(ctx, in.Cursor, limit, in.Query, "", "")
	if err != nil {
		return nil, nil, fmt.Errorf("list files: %w", err)
	}

	out := listOutput{Files: make([]compactFile, 0, len(resp.Items))}
	for _, f := range resp.Items {
		out.Files = append(out.Files, compactFile{
			ID:        f.ID,
			Filename:  f.OriginalFilename,
			Size:      f.SizeBytes,
			CreatedAt: f.CreatedAt,
		})
	}
	out.Count = len(out.Files)
	if resp.NextCursor != nil {
		out.NextCursor = *resp.NextCursor
	}

	data, err := json.Marshal(out)
	if err != nil {
		return nil, nil, fmt.Errorf("encode list result: %w", err)
	}
	return textResult(string(data)), nil, nil
}
