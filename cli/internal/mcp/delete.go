package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const deleteDescription = "Delete a PushKit file. Safety-gated: without confirm=true it returns the file that WOULD be deleted (a dry run) and deletes nothing; pass confirm=true to actually delete. Select by id or by exact filename (newest match)."

type deleteInput struct {
	ID       string `json:"id,omitempty" jsonschema:"the file ID to delete"`
	Filename string `json:"filename,omitempty" jsonschema:"delete the newest file with this exact filename"`
	Confirm  bool   `json:"confirm,omitempty" jsonschema:"must be true to actually delete; false (default) returns a dry-run preview"`
}

type deleteOutput struct {
	Deleted  bool   `json:"deleted"`
	FileID   string `json:"file_id"`
	Filename string `json:"filename,omitempty"`
	Message  string `json:"message"`
}

func (h *handlers) delete(ctx context.Context, _ *mcp.CallToolRequest, in deleteInput) (*mcp.CallToolResult, any, error) {
	if err := h.requireKey(); err != nil {
		return nil, nil, err
	}

	id, filename, err := h.resolveDeleteTarget(ctx, in)
	if err != nil {
		return nil, nil, err
	}

	if !in.Confirm {
		return jsonResult(deleteOutput{
			Deleted:  false,
			FileID:   id,
			Filename: filename,
			Message:  "not deleted — re-call pushkit_delete with confirm=true to delete this file",
		})
	}

	if err := h.c.Delete(ctx, id); err != nil {
		return nil, nil, fmt.Errorf("delete %s: %w", id, err)
	}
	return jsonResult(deleteOutput{
		Deleted:  true,
		FileID:   id,
		Filename: filename,
		Message:  "file deleted",
	})
}

// resolveDeleteTarget resolves the selector to a single file id (and filename
// when known). Exactly one of id or filename must be given.
func (h *handlers) resolveDeleteTarget(ctx context.Context, in deleteInput) (id, filename string, err error) {
	switch {
	case in.ID != "" && in.Filename != "":
		return "", "", fmt.Errorf("specify either id or filename, not both")
	case in.ID != "":
		return in.ID, "", nil
	case in.Filename != "":
		resp, err := h.c.ListFiles(ctx, "", maxListLimit, in.Filename, "", "")
		if err != nil {
			return "", "", fmt.Errorf("list files: %w", err)
		}
		for _, f := range resp.Items { // newest-first
			if f.OriginalFilename == in.Filename {
				return f.ID, f.OriginalFilename, nil
			}
		}
		return "", "", fmt.Errorf("no file named %q found", in.Filename)
	default:
		return "", "", fmt.Errorf("specify id or filename to delete")
	}
}

func jsonResult(v any) (*mcp.CallToolResult, any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, nil, fmt.Errorf("encode result: %w", err)
	}
	return textResult(string(data)), nil, nil
}
