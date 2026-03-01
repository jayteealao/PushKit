package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSON_OversizedBody(t *testing.T) {
	// Build a body larger than maxRequestBody (1 MB).
	oversized := strings.Repeat("x", maxRequestBody+1)
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(oversized))
	w := httptest.NewRecorder()

	var out map[string]any
	err := decodeJSON(w, r, &out)
	if err == nil {
		t.Fatal("expected error for oversized body, got nil")
	}
}

func TestDecodeJSON_UnknownFieldsAccepted(t *testing.T) {
	// Ensure that extra/unknown fields do NOT cause an error (forward compatibility).
	body := `{"filename":"test.pdf","contentType":"text/plain","unknownField":true}`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()

	type req struct {
		Filename    string `json:"filename"`
		ContentType string `json:"contentType"`
	}
	var out req
	err := decodeJSON(w, r, &out)
	if err != nil {
		t.Fatalf("expected unknown fields to be accepted, got error: %v", err)
	}
	if out.Filename != "test.pdf" {
		t.Errorf("expected filename=test.pdf, got %q", out.Filename)
	}
}

func TestDecodeJSON_EmptyBody(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()

	var out map[string]any
	err := decodeJSON(w, r, &out)
	if err == nil {
		t.Fatal("expected error for nil body, got nil")
	}
}
