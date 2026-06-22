package mcp

import (
	"context"
	"io"

	"github.com/pushkit/cli/internal/client"
)

// stubClient is an in-memory apiClient for the push/list/delete handler tests.
// Each method delegates to an injected func when set, else returns a sensible
// default. Delete calls are recorded so the orphan-cleanup path can be asserted.
type stubClient struct {
	initFn     func(context.Context, *client.UploadInitRequest) (*client.UploadInitResponse, error)
	completeFn func(context.Context, *client.UploadCompleteRequest) (*client.FileResponse, error)
	putFn      func(context.Context, string, io.Reader, string, int64) error
	listFn     func(context.Context, string, int, string, string, string) (*client.FileListResponse, error)
	downloadFn func(context.Context, string) (*client.DownloadResponse, error)
	deleteFn   func(context.Context, string) error

	deleted []string // fileIDs passed to Delete, in call order
}

func (s *stubClient) InitUpload(ctx context.Context, req *client.UploadInitRequest) (*client.UploadInitResponse, error) {
	if s.initFn != nil {
		return s.initFn(ctx, req)
	}
	return &client.UploadInitResponse{
		FileID:          "file-" + req.Filename,
		PresignedPutURL: "http://put.invalid/object",
		RequiredHeaders: map[string]string{"Content-Type": req.ContentType},
	}, nil
}

func (s *stubClient) CompleteUpload(ctx context.Context, req *client.UploadCompleteRequest) (*client.FileResponse, error) {
	if s.completeFn != nil {
		return s.completeFn(ctx, req)
	}
	return &client.FileResponse{ID: req.FileID, Status: "UPLOADED"}, nil
}

func (s *stubClient) PutToPresignedURL(ctx context.Context, url string, body io.Reader, contentType string, size int64) error {
	if s.putFn != nil {
		return s.putFn(ctx, url, body, contentType, size)
	}
	_, _ = io.Copy(io.Discard, body) // mimic consuming the upload body
	return nil
}

func (s *stubClient) ListFiles(ctx context.Context, cursor string, limit int, search, sort, order string) (*client.FileListResponse, error) {
	if s.listFn != nil {
		return s.listFn(ctx, cursor, limit, search, sort, order)
	}
	return &client.FileListResponse{}, nil
}

func (s *stubClient) GetDownloadURL(ctx context.Context, fileID string) (*client.DownloadResponse, error) {
	if s.downloadFn != nil {
		return s.downloadFn(ctx, fileID)
	}
	return &client.DownloadResponse{}, nil
}

func (s *stubClient) Delete(ctx context.Context, fileID string) error {
	s.deleted = append(s.deleted, fileID)
	if s.deleteFn != nil {
		return s.deleteFn(ctx, fileID)
	}
	return nil
}
