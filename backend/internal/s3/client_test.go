package s3

import (
	"context"
	"net/http"
	"testing"

	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
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
