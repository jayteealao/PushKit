package db

import (
	"database/sql"
	"testing"
	"time"

	"github.com/pushkit/backend/internal/models"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := CreateTables(db); err != nil {
		t.Fatalf("create tables: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestCreateTables(t *testing.T) {
	db := setupTestDB(t)

	// Verify files table exists.
	var name string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='files'").Scan(&name)
	if err != nil {
		t.Fatalf("files table not found: %v", err)
	}
}

func TestInsertAndGetFile(t *testing.T) {
	database := setupTestDB(t)

	size := int64(1234)
	f := &models.FileRecord{
		ID:               "test-id-1",
		UserID:           "user1",
		S3Key:            "uploads/user1/2026/03/01/abc-test.pdf",
		OriginalFilename: "test.pdf",
		ContentType:      "application/pdf",
		SizeBytes:        &size,
		CreatedAt:        time.Now().UTC(),
		Status:           models.StatusInitiated,
	}

	if err := InsertFile(database, f); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := GetFileByIDAndUser(database, "test-id-1", "user1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.OriginalFilename != "test.pdf" {
		t.Errorf("filename: got %q, want test.pdf", got.OriginalFilename)
	}
	if *got.SizeBytes != 1234 {
		t.Errorf("size: got %d, want 1234", *got.SizeBytes)
	}
}

func TestGetFile_WrongUser(t *testing.T) {
	database := setupTestDB(t)

	f := &models.FileRecord{
		ID:               "test-id-2",
		UserID:           "user1",
		S3Key:            "uploads/user1/2026/03/01/def-test.pdf",
		OriginalFilename: "test.pdf",
		ContentType:      "application/pdf",
		CreatedAt:        time.Now().UTC(),
		Status:           models.StatusInitiated,
	}
	InsertFile(database, f)

	_, err := GetFileByIDAndUser(database, "test-id-2", "user2")
	if err != sql.ErrNoRows {
		t.Errorf("expected ErrNoRows, got %v", err)
	}
}

func TestUpdateFileStatus(t *testing.T) {
	database := setupTestDB(t)

	f := &models.FileRecord{
		ID:               "test-id-3",
		UserID:           "user1",
		S3Key:            "uploads/user1/2026/03/01/ghi-test.pdf",
		OriginalFilename: "test.pdf",
		ContentType:      "application/pdf",
		CreatedAt:        time.Now().UTC(),
		Status:           models.StatusInitiated,
	}
	InsertFile(database, f)

	size := int64(5678)
	sha := "abc123"
	if err := UpdateFileStatus(database, "test-id-3", models.StatusUploaded, &size, &sha, nil); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := GetFileByIDAndUser(database, "test-id-3", "user1")
	if got.Status != models.StatusUploaded {
		t.Errorf("status: got %q, want UPLOADED", got.Status)
	}
	if *got.SizeBytes != 5678 {
		t.Errorf("size: got %d, want 5678", *got.SizeBytes)
	}
}

func TestListFiles_OnlyUploaded(t *testing.T) {
	database := setupTestDB(t)

	now := time.Now().UTC()
	// Insert an INITIATED file (should be excluded from list).
	InsertFile(database, &models.FileRecord{
		ID: "init-1", UserID: "user1", S3Key: "uploads/user1/a",
		OriginalFilename: "a.pdf", ContentType: "application/pdf",
		CreatedAt: now, Status: models.StatusInitiated,
	})
	// Insert an UPLOADED file (should be included).
	InsertFile(database, &models.FileRecord{
		ID: "uploaded-1", UserID: "user1", S3Key: "uploads/user1/b",
		OriginalFilename: "b.pdf", ContentType: "application/pdf",
		CreatedAt: now.Add(time.Second), Status: models.StatusUploaded,
	})
	// Insert another user's UPLOADED file (should be excluded).
	InsertFile(database, &models.FileRecord{
		ID: "other-1", UserID: "user2", S3Key: "uploads/user2/c",
		OriginalFilename: "c.pdf", ContentType: "application/pdf",
		CreatedAt: now.Add(2 * time.Second), Status: models.StatusUploaded,
	})

	files, nextCursor, err := ListFiles(database, ListParams{
		UserID: "user1", Limit: 20,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].ID != "uploaded-1" {
		t.Errorf("expected uploaded-1, got %s", files[0].ID)
	}
	if nextCursor != nil {
		t.Error("expected no next cursor")
	}
}

func TestListFiles_Pagination(t *testing.T) {
	database := setupTestDB(t)

	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		InsertFile(database, &models.FileRecord{
			ID: "page-" + string(rune('a'+i)), UserID: "user1",
			S3Key:            "uploads/user1/page-" + string(rune('a'+i)),
			OriginalFilename: "file" + string(rune('a'+i)) + ".pdf",
			ContentType:      "application/pdf",
			CreatedAt:        now.Add(time.Duration(i) * time.Second),
			Status:           models.StatusUploaded,
		})
	}

	files, nextCursor, err := ListFiles(database, ListParams{
		UserID: "user1", Limit: 3,
	})
	if err != nil {
		t.Fatalf("list page 1: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("page 1: expected 3, got %d", len(files))
	}
	if nextCursor == nil {
		t.Fatal("expected next cursor")
	}

	files2, nextCursor2, err := ListFiles(database, ListParams{
		UserID: "user1", Limit: 3, Cursor: *nextCursor,
	})
	if err != nil {
		t.Fatalf("list page 2: %v", err)
	}
	if len(files2) != 2 {
		t.Fatalf("page 2: expected 2, got %d", len(files2))
	}
	if nextCursor2 != nil {
		t.Error("expected no next cursor on last page")
	}
}

func TestDeleteFile(t *testing.T) {
	database := setupTestDB(t)

	InsertFile(database, &models.FileRecord{
		ID: "del-1", UserID: "user1", S3Key: "uploads/user1/del",
		OriginalFilename: "del.pdf", ContentType: "application/pdf",
		CreatedAt: time.Now().UTC(), Status: models.StatusUploaded,
	})

	if err := DeleteFile(database, "del-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := GetFileByID(database, "del-1")
	if err != sql.ErrNoRows {
		t.Errorf("expected ErrNoRows after delete, got %v", err)
	}
}

func TestUniqueS3Key(t *testing.T) {
	database := setupTestDB(t)

	f := &models.FileRecord{
		ID: "dup-1", UserID: "user1", S3Key: "uploads/user1/same-key",
		OriginalFilename: "a.pdf", ContentType: "application/pdf",
		CreatedAt: time.Now().UTC(), Status: models.StatusInitiated,
	}
	InsertFile(database, f)

	f2 := &models.FileRecord{
		ID: "dup-2", UserID: "user1", S3Key: "uploads/user1/same-key",
		OriginalFilename: "b.pdf", ContentType: "application/pdf",
		CreatedAt: time.Now().UTC(), Status: models.StatusInitiated,
	}
	err := InsertFile(database, f2)
	if err == nil {
		t.Error("expected error on duplicate s3_key")
	}
}
