package s3

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/pushkit/backend/internal/config"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"report.pdf", "report.pdf"},
		{"my file (1).pdf", "my_file__1_.pdf"},
		{"../../etc/passwd", ".._.._etc_passwd"},
		{"path/to\\file.txt", "path_to_file.txt"},
		{"   spaces   ", "spaces"},
		{"", "file"},
		{"hello world!@#$.txt", "hello_world____.txt"},
		{"résumé.pdf", "r_sum_.pdf"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SanitizeFilename(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildS3Key_Format(t *testing.T) {
	key := BuildS3Key("user1", "report.pdf")

	if len(key) == 0 {
		t.Fatal("expected non-empty key")
	}

	// Must start with uploads/user1/
	prefix := "uploads/user1/"
	if key[:len(prefix)] != prefix {
		t.Errorf("expected key to start with %q, got %q", prefix, key)
	}

	// Must end with sanitized filename.
	if key[len(key)-len("report.pdf"):] != "report.pdf" {
		t.Errorf("expected key to end with report.pdf, got %q", key)
	}
}

func TestSanitizeFilename_LongName(t *testing.T) {
	long := ""
	for i := 0; i < 200; i++ {
		long += "a"
	}
	key := BuildS3Key("user1", long)
	// The sanitized portion should be truncated.
	if len(key) > 300 {
		t.Errorf("key too long: %d", len(key))
	}
}

// stubFinalizeHandler is a no-op FinalizeHandler used in middleware unit tests.
type stubFinalizeHandler struct{}

func (stubFinalizeHandler) HandleFinalize(_ context.Context, in middleware.FinalizeInput) (middleware.FinalizeOutput, middleware.Metadata, error) {
	return middleware.FinalizeOutput{Result: in.Request}, middleware.Metadata{}, nil
}

// newSmithyRequest builds a *smithyhttp.Request with the given headers preset.
func newSmithyRequest(headers map[string]string) *smithyhttp.Request {
	r := smithyhttp.NewStackRequest().(*smithyhttp.Request)
	// Replace with a real *http.Request so headers work as expected.
	r.Request = &http.Request{Header: make(http.Header)}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

// TestStripSDKHeaders verifies the pure helper removes exactly the three
// targeted headers and leaves others intact.
func TestStripSDKHeaders(t *testing.T) {
	req := newSmithyRequest(map[string]string{
		"Amz-Sdk-Invocation-Id": "abc-123",
		"Amz-Sdk-Request":       "attempt=1",
		"Accept-Encoding":       "gzip",
		"Content-Type":          "application/octet-stream", // must be preserved
	})

	stripSDKHeaders(req)

	for _, h := range []string{"Amz-Sdk-Invocation-Id", "Amz-Sdk-Request", "Accept-Encoding"} {
		if v := req.Header.Get(h); v != "" {
			t.Errorf("header %q should have been removed, got %q", h, v)
		}
	}
	if got := req.Header.Get("Content-Type"); got != "application/octet-stream" {
		t.Errorf("Content-Type should be preserved, got %q", got)
	}
}

// TestStripSDKHeadersMiddleware_RemovesHeaders verifies that the middleware
// constructor correctly wraps stripSDKHeaders and passes the request through.
func TestStripSDKHeadersMiddleware_RemovesHeaders(t *testing.T) {
	req := newSmithyRequest(map[string]string{
		"Amz-Sdk-Invocation-Id": "inv-id-1",
		"Amz-Sdk-Request":       "attempt=1; max=3",
		"Accept-Encoding":       "gzip",
		"X-Custom-Header":       "keep-me",
	})

	m := stripSDKHeadersMiddleware()

	_, _, err := m.HandleFinalize(
		context.Background(),
		middleware.FinalizeInput{Request: req},
		stubFinalizeHandler{},
	)
	if err != nil {
		t.Fatalf("middleware returned unexpected error: %v", err)
	}

	for _, h := range []string{"Amz-Sdk-Invocation-Id", "Amz-Sdk-Request", "Accept-Encoding"} {
		if v := req.Header.Get(h); v != "" {
			t.Errorf("header %q should have been stripped by middleware, got %q", h, v)
		}
	}
	if got := req.Header.Get("X-Custom-Header"); got != "keep-me" {
		t.Errorf("X-Custom-Header should be preserved, got %q", got)
	}
}

// TestStripSDKHeadersMiddleware_NonSmithyRequest verifies the middleware is a
// no-op (no panic) when the request is not a *smithyhttp.Request.
func TestStripSDKHeadersMiddleware_NonSmithyRequest(t *testing.T) {
	m := stripSDKHeadersMiddleware()
	// Pass a plain string as the request — should not panic or error.
	_, _, err := m.HandleFinalize(
		context.Background(),
		middleware.FinalizeInput{Request: "not-a-smithy-request"},
		stubFinalizeHandler{},
	)
	if err != nil {
		t.Fatalf("middleware should not error on non-smithy request: %v", err)
	}
}

// TestGeneratePresignURLs_S3CompatibleMode is the regression guard for the
// S3-compatible presign break. A client configured with S3_ENDPOINT_URL set
// (MinIO/R2/GCS mode) must presign both a PUT and a GET without error, and the
// resulting URLs must NOT sign the stripped SDK telemetry headers.
//
// Pre-fix — when the strip middleware was registered with
// Finalize.Insert(…, "Signing", Before) — this fails with
// "presign PUT: ... not found: StripSDKHeaders", because the presign pipeline
// has no "Signing" step. Presigning makes no network calls (static creds +
// explicit region), so no live S3/MinIO server is required.
func TestGeneratePresignURLs_S3CompatibleMode(t *testing.T) {
	cfg := &config.Config{
		AWSRegion:      "us-east-1",
		AWSAccessKeyID: "test-access-key-id",
		AWSSecretKey:   "test-secret-access-key",
		S3EndpointURL:  "http://127.0.0.1:9000", // S3-compatible mode (e.g. MinIO)
		S3Bucket:       "test-bucket",
		PresignTTLSecs: 900,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	putURL, err := client.GenerateUploadURL(context.Background(), "uploads/u1/file.bin", "application/octet-stream")
	if err != nil {
		t.Fatalf("GenerateUploadURL (S3-compatible presign): %v", err)
	}
	getURL, err := client.GenerateDownloadURL(context.Background(), "uploads/u1/file.bin", "file.bin")
	if err != nil {
		t.Fatalf("GenerateDownloadURL (S3-compatible presign): %v", err)
	}

	// The three SDK headers are injected in the Build step; the strip middleware
	// must remove them before signing so they never appear in the presigned
	// URL's X-Amz-SignedHeaders. If they leaked into the signed set, a non-SDK
	// client (Android, curl, the MCP CLI) PUT/GET to that URL would 403 with
	// SignatureDoesNotMatch. This also guards against an "errors-free but strips
	// nothing" regression.
	strippedHeaders := []string{"amz-sdk-invocation-id", "amz-sdk-request", "accept-encoding"}
	for _, tc := range []struct {
		name   string
		rawURL string
	}{
		{"PUT", putURL},
		{"GET", getURL},
	} {
		u, err := url.Parse(tc.rawURL)
		if err != nil {
			t.Fatalf("%s: parse presigned URL: %v", tc.name, err)
		}
		signed := strings.ToLower(u.Query().Get("X-Amz-SignedHeaders"))
		if signed == "" {
			t.Fatalf("%s: presigned URL has no X-Amz-SignedHeaders: %s", tc.name, tc.rawURL)
		}
		for _, h := range strippedHeaders {
			if strings.Contains(signed, h) {
				t.Errorf("%s: X-Amz-SignedHeaders must omit %q, got %q", tc.name, h, signed)
			}
		}
	}
}
