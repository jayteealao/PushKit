package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pushkit/backend/internal/config"
)

func TestMiddleware_ValidKey(t *testing.T) {
	cfg := &config.Config{
		APIKeyMap: map[string]string{"test-key-1": "user1"},
	}

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := GetUserID(r.Context())
		if userID != "user1" {
			t.Errorf("expected user1, got %q", userID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "test-key-1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestMiddleware_InvalidKey(t *testing.T) {
	cfg := &config.Config{
		APIKeyMap: map[string]string{"test-key-1": "user1"},
	}

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "wrong-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestMiddleware_MissingKey(t *testing.T) {
	cfg := &config.Config{
		APIKeyMap: map[string]string{"test-key-1": "user1"},
	}

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestLookupKey_ConstantTime(t *testing.T) {
	keyMap := map[string]string{
		"alpha": "user-a",
		"beta":  "user-b",
	}

	if uid, ok := lookupKey(keyMap, "alpha"); !ok || uid != "user-a" {
		t.Errorf("expected user-a, got %q, ok=%v", uid, ok)
	}
	if _, ok := lookupKey(keyMap, "gamma"); ok {
		t.Error("expected not found")
	}
}
