package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDelete(t *testing.T) {
	tests := []struct {
		name       string
		fileID     string
		status     int
		wantPath   string
		wantErr    bool
	}{
		{name: "success 204", fileID: "abc-123", status: http.StatusNoContent, wantPath: "/v1/files/abc-123", wantErr: false},
		{name: "not found 404", fileID: "missing", status: http.StatusNotFound, wantPath: "/v1/files/missing", wantErr: true},
		{name: "server error 500", fileID: "boom", status: http.StatusInternalServerError, wantPath: "/v1/files/boom", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath, gotKey string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				gotKey = r.Header.Get("X-API-Key")
				if tt.status == http.StatusNoContent {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(`{"error":"boom"}`))
			}))
			defer srv.Close()

			c := New(srv.URL, "test-key")
			err := c.Delete(context.Background(), tt.fileID)

			if tt.wantErr && err == nil {
				t.Fatalf("expected an error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotMethod != http.MethodDelete {
				t.Errorf("method = %q, want DELETE", gotMethod)
			}
			if gotPath != tt.wantPath {
				t.Errorf("path = %q, want %q", gotPath, tt.wantPath)
			}
			if gotKey != "test-key" {
				t.Errorf("X-API-Key = %q, want %q", gotKey, "test-key")
			}
		})
	}
}
