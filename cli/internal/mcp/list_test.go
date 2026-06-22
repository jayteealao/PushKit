package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/pushkit/cli/internal/client"
)

// resultText extracts the single text block from a tool result.
func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if res == nil || len(res.Content) == 0 {
		t.Fatal("result has no content")
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] is %T, want *mcp.TextContent", res.Content[0])
	}
	return tc.Text
}

func TestListCompactAndPaginated(t *testing.T) {
	size := int64(123)
	next := "cursor-2"
	var gotCursor string
	var gotLimit int

	stub := &stubClient{
		listFn: func(_ context.Context, cursor string, limit int, _, _, _ string) (*client.FileListResponse, error) {
			gotCursor, gotLimit = cursor, limit
			return &client.FileListResponse{
				Items: []client.FileResponse{{
					ID:               "id1",
					OriginalFilename: "a.txt",
					SizeBytes:        &size,
					CreatedAt:        "2026-01-01T00:00:00Z",
					ContentType:      "text/plain",
					Status:           "UPLOADED",
				}},
				NextCursor: &next,
			}, nil
		},
	}
	h := &handlers{c: stub, apiKey: "k"}

	res, _, err := h.list(context.Background(), nil, listInput{Cursor: "cursor-1", Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	text := resultText(t, res)

	// Compact fields present.
	for _, want := range []string{`"id":"id1"`, `"filename":"a.txt"`, `"size":123`, `"created_at":"2026-01-01T00:00:00Z"`, `"next_cursor":"cursor-2"`} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %s\ngot: %s", want, text)
		}
	}
	// Non-compact fields must NOT leak.
	for _, bad := range []string{"originalFilename", `"status"`, "contentType", "UPLOADED", "s3Key"} {
		if strings.Contains(text, bad) {
			t.Errorf("output leaked non-compact field %q\ngot: %s", bad, text)
		}
	}
	// Cursor and limit were passed through to the client.
	if gotCursor != "cursor-1" {
		t.Errorf("cursor passthrough = %q, want cursor-1", gotCursor)
	}
	if gotLimit != 10 {
		t.Errorf("limit passthrough = %d, want 10", gotLimit)
	}
}

func TestListLimitClamped(t *testing.T) {
	var gotLimit int
	stub := &stubClient{
		listFn: func(_ context.Context, _ string, limit int, _, _, _ string) (*client.FileListResponse, error) {
			gotLimit = limit
			return &client.FileListResponse{}, nil
		},
	}
	h := &handlers{c: stub, apiKey: "k"}

	if _, _, err := h.list(context.Background(), nil, listInput{Limit: 9999}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if gotLimit != maxListLimit {
		t.Errorf("limit = %d, want clamped to %d", gotLimit, maxListLimit)
	}
}
