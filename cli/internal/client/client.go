package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	apiKey     string
	http       *http.Client // authenticated backend API calls
	httpUpload *http.Client // credential-free client for presigned S3 PUTs
}

// newUploadClient returns a dedicated http.Client for presigned S3 PUT requests.
// It carries no auth headers, no API key logging, and has no absolute timeout
// (uploads are bounded by per-request context deadlines instead). TLS is floored
// at 1.2 as defense-in-depth.
func newUploadClient() *http.Client {
	return &http.Client{
		Timeout: 0, // no absolute deadline; callers use per-request context timeouts
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout: 10 * time.Second,
			TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}
}

// newAPIClient returns the http.Client used for authenticated backend API calls.
// No absolute Timeout is set: that is a deadline from request-start and would
// kill CompleteUpload (which triggers a backend S3 HeadObject that can be slow).
// Individual slow paths must be bounded by their caller's context deadline.
// The Transport-level timeouts (dial, TLS, response-headers) still guard against
// hung connections. TLS is floored at 1.2 as defense-in-depth.
func newAPIClient() *http.Client {
	return &http.Client{
		Timeout: 0,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		http:       newAPIClient(),
		httpUpload: newUploadClient(),
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

// Delete removes a file by ID via DELETE /v1/files/{fileID}. The backend
// hard-deletes the row regardless of status (so an orphaned INITIATED upload can
// be cleaned up) and best-effort removes the S3 object. A 204 No Content is
// success; a non-2xx response is mapped to an error by doJSON.
func (c *Client) Delete(ctx context.Context, fileID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/v1/files/"+url.PathEscape(fileID), nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-API-Key", c.apiKey)
	return c.doJSON(req, nil)
}

// PutToPresignedURL uploads data to a presigned S3 URL using a credential-free
// http.Client — the presigned URL is self-authenticating and must not carry the
// backend API key or any other auth header.
func (c *Client) PutToPresignedURL(ctx context.Context, presignedURL string, body io.Reader, contentType string, size int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, presignedURL, body)
	if err != nil {
		return fmt.Errorf("create PUT request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.ContentLength = size

	resp, err := c.httpUpload.Do(req)
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
