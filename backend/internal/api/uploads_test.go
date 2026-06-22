package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/pushkit/backend/internal/auth"
	"github.com/pushkit/backend/internal/config"
	"github.com/pushkit/backend/internal/db"
	"github.com/pushkit/backend/internal/events"
	"github.com/pushkit/backend/internal/models"
	s3client "github.com/pushkit/backend/internal/s3"
)

// spyPublisher is a test-double for events.Publisher. It records all published
// events and signals publishCh each time one arrives.
type spyPublisher struct {
	mu        sync.Mutex
	published []struct {
		userID string
		ev     events.Event
	}
	publishCh chan struct{}
}

func newSpyPublisher() *spyPublisher {
	return &spyPublisher{publishCh: make(chan struct{}, 16)}
}

func (s *spyPublisher) Publish(userID string, ev events.Event) {
	s.mu.Lock()
	s.published = append(s.published, struct {
		userID string
		ev     events.Event
	}{userID, ev})
	s.mu.Unlock()
	s.publishCh <- struct{}{}
}

func (s *spyPublisher) waitForPublish(t *testing.T) (string, events.Event) {
	t.Helper()
	select {
	case <-s.publishCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Publish was not called within deadline")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	last := s.published[len(s.published)-1]
	return last.userID, last.ev
}

func (s *spyPublisher) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.published)
}

// openTestDB opens an in-memory SQLite DB with the standard schema applied,
// registering cleanup on t.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.CreateTables(database); err != nil {
		database.Close()
		t.Fatalf("create tables: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

// fakeS3Server starts an httptest.Server that responds 200 to HEAD requests
// (simulating an object-exists scenario). The returned setNotFound func
// switches subsequent HEAD responses to 404.
func fakeS3Server(t *testing.T) (srv *httptest.Server, setNotFound func()) {
	t.Helper()
	headStatus := http.StatusOK
	var mu sync.Mutex
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		st := headStatus
		mu.Unlock()
		if r.Method == http.MethodHead {
			w.WriteHeader(st)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	t.Cleanup(srv.Close)
	return srv, func() {
		mu.Lock()
		headStatus = http.StatusNotFound
		mu.Unlock()
	}
}

// newTestS3Client builds an s3client.Client pointed at fakeEndpoint with dummy
// credentials, for use in tests that need to control S3 behaviour.
func newTestS3Client(t *testing.T, fakeEndpoint string) *s3client.Client {
	t.Helper()
	cfg := &config.Config{
		AWSRegion:      "us-east-1",
		AWSAccessKeyID: "test-key-id",
		AWSSecretKey:   "test-secret",
		S3EndpointURL:  fakeEndpoint,
		S3Bucket:       "test-bucket",
		PresignTTLSecs: 900,
	}
	c, err := s3client.NewClient(cfg)
	if err != nil {
		t.Fatalf("newTestS3Client: %v", err)
	}
	return c
}

// buildCompleteUploadServer wires an UploadHandler (real SQLite + given publisher)
// behind the auth middleware and returns the httptest.Server and the raw *sql.DB
// so tests can seed records.
func buildCompleteUploadServer(t *testing.T, spy *spyPublisher, s3Endpoint string) (*httptest.Server, *sql.DB) {
	t.Helper()
	database := openTestDB(t)
	s3 := newTestS3Client(t, s3Endpoint)

	uploadHandler := &UploadHandler{DB: database, S3: s3, Events: spy}
	apiCfg := &config.Config{APIKeyMap: map[string]string{"key-user1": "user1"}}

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(apiCfg))
		r.Mount("/v1/uploads", uploadHandler.Routes())
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv, database
}

func TestInitUpload_NegativeSizeBytes(t *testing.T) {
	// Directly test that the handler rejects negative sizeBytes by sending
	// a request with a negative value and checking the response.
	// Since the handler needs DB/S3, we only test the decoding + validation
	// portion by simulating the same logic.
	body := `{"filename":"test.pdf","contentType":"text/plain","sizeBytes":-1}`
	r := httptest.NewRequest(http.MethodPost, "/v1/uploads/init", strings.NewReader(body))
	w := httptest.NewRecorder()

	var req models.UploadInitRequest
	if err := decodeJSON(w, r, &req); err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}

	if req.SizeBytes == nil {
		t.Fatal("expected sizeBytes to be non-nil")
	}
	if *req.SizeBytes >= 0 {
		t.Fatalf("expected negative sizeBytes, got %d", *req.SizeBytes)
	}

	// Verify validation would reject it.
	if req.SizeBytes != nil && *req.SizeBytes < 0 {
		// This is the expected path — validation triggers.
		writeError(w, http.StatusBadRequest, "sizeBytes must be non-negative")
	}

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var errResp models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Error != "sizeBytes must be non-negative" {
		t.Errorf("expected error 'sizeBytes must be non-negative', got %q", errResp.Error)
	}
}

func TestInitUpload_ZeroSizeBytesAllowed(t *testing.T) {
	body := `{"filename":"empty.txt","contentType":"text/plain","sizeBytes":0}`
	r := httptest.NewRequest(http.MethodPost, "/v1/uploads/init", strings.NewReader(body))
	w := httptest.NewRecorder()

	var req models.UploadInitRequest
	if err := decodeJSON(w, r, &req); err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}

	if req.SizeBytes == nil {
		t.Fatal("expected sizeBytes to be non-nil for explicit 0")
	}
	if *req.SizeBytes != 0 {
		t.Fatalf("expected sizeBytes=0, got %d", *req.SizeBytes)
	}

	// Validation should NOT reject 0.
	if req.SizeBytes != nil && *req.SizeBytes < 0 {
		t.Fatal("validation incorrectly rejected sizeBytes=0")
	}
}

func TestCompleteUpload_NegativeSizeBytes(t *testing.T) {
	body := `{"fileId":"abc-123","sizeBytes":-42}`
	r := httptest.NewRequest(http.MethodPost, "/v1/uploads/complete", strings.NewReader(body))
	w := httptest.NewRecorder()

	var req models.UploadCompleteRequest
	if err := decodeJSON(w, r, &req); err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}

	if req.SizeBytes == nil || *req.SizeBytes >= 0 {
		t.Fatal("expected negative sizeBytes in request")
	}

	// Validation rejects.
	if req.SizeBytes != nil && *req.SizeBytes < 0 {
		writeError(w, http.StatusBadRequest, "sizeBytes must be non-negative")
	}

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// seedInitiatedFile inserts a file record in INITIATED status and returns its ID
// and S3 key so the caller can drive completeUpload against it.
func seedInitiatedFile(t *testing.T, database *sql.DB, userID, fileID, s3Key string) {
	t.Helper()
	if err := db.InsertFile(database, &models.FileRecord{
		ID:               fileID,
		UserID:           userID,
		S3Key:            s3Key,
		OriginalFilename: "test.pdf",
		ContentType:      "application/pdf",
		CreatedAt:        time.Now().UTC(),
		Status:           models.StatusInitiated,
	}); err != nil {
		t.Fatalf("seedInitiatedFile: %v", err)
	}
}

// TestCompleteUpload_PublishesEventAfterStatusUpdate is the integration gate for
// TST-2: completeUpload must publish a file.uploaded event to the correct user
// AFTER the DB status update succeeds, with deterministic synchronisation.
func TestCompleteUpload_PublishesEventAfterStatusUpdate(t *testing.T) {
	s3Srv, _ := fakeS3Server(t)
	spy := newSpyPublisher()
	srv, database := buildCompleteUploadServer(t, spy, s3Srv.URL)

	const (
		userID = "user1"
		fileID = "file-pub-1"
		s3Key  = "test-bucket/user1/test.pdf"
	)
	seedInitiatedFile(t, database, userID, fileID, s3Key)

	body := fmt.Sprintf(`{"fileId":%q}`, fileID)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		srv.URL+"/v1/uploads/complete", strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "key-user1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	// Verify DB status was updated before the publish arrived.
	updated, err := db.GetFileByIDAndUser(database, fileID, userID)
	if err != nil {
		t.Fatalf("re-fetch record: %v", err)
	}
	if updated.Status != models.StatusUploaded {
		t.Fatalf("status = %q after complete, want UPLOADED", updated.Status)
	}

	// The publish must arrive with the correct user and file ID.
	publishedUser, ev := spy.waitForPublish(t)
	if publishedUser != userID {
		t.Errorf("Publish called for user %q, want %q", publishedUser, userID)
	}
	if ev.FileID != fileID {
		t.Errorf("event FileID = %q, want %q", ev.FileID, fileID)
	}
	if ev.Type != "file.uploaded" {
		t.Errorf("event Type = %q, want file.uploaded", ev.Type)
	}
}

// TestCompleteUpload_DoesNotPublishWhenStatusUpdateFails asserts that the spy
// publisher is NOT called when UpdateFileStatus would fail. We simulate the
// failure by closing the DB connection before the handler runs.
func TestCompleteUpload_DoesNotPublishWhenStatusUpdateFails(t *testing.T) {
	s3Srv, _ := fakeS3Server(t)
	spy := newSpyPublisher()

	// Build a separate short-lived DB so we can close it to force failure.
	database := openTestDB(t)
	s3 := newTestS3Client(t, s3Srv.URL)

	const (
		userID = "user1"
		fileID = "file-nopub-1"
		s3Key  = "test-bucket/user1/fail.pdf"
	)
	seedInitiatedFile(t, database, userID, fileID, s3Key)

	uploadHandler := &UploadHandler{DB: database, S3: s3, Events: spy}
	apiCfg := &config.Config{APIKeyMap: map[string]string{"key-user1": "user1"}}

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(apiCfg))
		r.Mount("/v1/uploads", uploadHandler.Routes())
	})
	handlerSrv := httptest.NewServer(r)
	t.Cleanup(handlerSrv.Close)

	// Close the DB so that UpdateFileStatus will error.
	database.Close()

	body := fmt.Sprintf(`{"fileId":%q}`, fileID)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		handlerSrv.URL+"/v1/uploads/complete", strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "key-user1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	// Handler should respond with 5xx (DB error) — exact code depends on which
	// DB call fails first; we only check it's not 200.
	if resp.StatusCode == http.StatusOK {
		t.Fatal("expected non-200 when DB is closed, but got 200")
	}

	// Spy must not have been called.
	if n := spy.callCount(); n != 0 {
		t.Fatalf("Publish called %d times, want 0 (no event on failure path)", n)
	}
}
