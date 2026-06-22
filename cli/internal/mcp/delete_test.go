package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/pushkit/cli/internal/client"
)

func parseDelete(t *testing.T, res *mcp.CallToolResult) deleteOutput {
	t.Helper()
	var out deleteOutput
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("decode delete output: %v", err)
	}
	return out
}

func TestDeleteRequiresConfirm(t *testing.T) {
	stub := &stubClient{}
	h := &handlers{c: stub, apiKey: "k"}

	res, _, err := h.delete(context.Background(), nil, deleteInput{ID: "id-1"})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	out := parseDelete(t, res)
	if out.Deleted {
		t.Error("delete without confirm must not delete")
	}
	if len(stub.deleted) != 0 {
		t.Errorf("no Delete call should have been made, got %v", stub.deleted)
	}
}

func TestDeleteWithConfirm(t *testing.T) {
	stub := &stubClient{}
	h := &handlers{c: stub, apiKey: "k"}

	res, _, err := h.delete(context.Background(), nil, deleteInput{ID: "id-1", Confirm: true})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	out := parseDelete(t, res)
	if !out.Deleted {
		t.Error("delete with confirm should delete")
	}
	if len(stub.deleted) != 1 || stub.deleted[0] != "id-1" {
		t.Errorf("expected Delete(id-1), got %v", stub.deleted)
	}
}

func TestDeleteByFilenameResolvesID(t *testing.T) {
	stub := &stubClient{
		listFn: func(_ context.Context, _ string, _ int, q, _, _ string) (*client.FileListResponse, error) {
			return &client.FileListResponse{Items: []client.FileResponse{
				{ID: "resolved-id", OriginalFilename: "doc.txt"},
			}}, nil
		},
	}
	h := &handlers{c: stub, apiKey: "k"}

	_, _, err := h.delete(context.Background(), nil, deleteInput{Filename: "doc.txt", Confirm: true})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(stub.deleted) != 1 || stub.deleted[0] != "resolved-id" {
		t.Errorf("filename should resolve to resolved-id, got %v", stub.deleted)
	}
}

func TestDeleteNoSelector(t *testing.T) {
	h := &handlers{c: &stubClient{}, apiKey: "k"}
	if _, _, err := h.delete(context.Background(), nil, deleteInput{}); err == nil {
		t.Error("expected an error with no selector")
	}
}
