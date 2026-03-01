package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{},
	}
}

func (c *Client) InitUpload(ctx context.Context, req *UploadInitRequest) (*UploadInitResponse, error) {
	var resp UploadInitResponse
	if err := c.post(ctx, "/v1/uploads/init", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) CompleteUpload(ctx context.Context, req *UploadCompleteRequest) (*FileResponse, error) {
	var resp FileResponse
	if err := c.post(ctx, "/v1/uploads/complete", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListFiles(ctx context.Context, cursor string, limit int, search, sort, order string) (*FileListResponse, error) {
	params := url.Values{}
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	if search != "" {
		params.Set("q", search)
	}
	if sort != "" {
		params.Set("sort", sort)
	}
	if order != "" {
		params.Set("order", order)
	}

	path := "/v1/files"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var resp FileListResponse
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetDownloadURL(ctx context.Context, fileID string) (*DownloadResponse, error) {
	var resp DownloadResponse
	if err := c.get(ctx, "/v1/files/"+fileID+"/download", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// PutToPresignedURL uploads data to a presigned S3 URL.
func (c *Client) PutToPresignedURL(ctx context.Context, presignedURL string, body io.Reader, contentType string, size int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, presignedURL, body)
	if err != nil {
		return fmt.Errorf("create PUT request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.ContentLength = size

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("PUT to S3: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("S3 PUT failed (HTTP %d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) post(ctx context.Context, path string, body, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	return c.doJSON(req, out)
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-API-Key", c.apiKey)

	return c.doJSON(req, out)
}

func (c *Client) doJSON(req *http.Request, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == 401 {
		return fmt.Errorf("authentication failed: invalid API key")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp ErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != "" {
			return fmt.Errorf("server error (HTTP %d): %s", resp.StatusCode, errResp.Error)
		}
		return fmt.Errorf("server error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}
