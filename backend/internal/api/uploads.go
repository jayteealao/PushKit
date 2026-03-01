package api

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/pushkit/backend/internal/auth"
	"github.com/pushkit/backend/internal/db"
	"github.com/pushkit/backend/internal/models"
	s3client "github.com/pushkit/backend/internal/s3"
)

type UploadHandler struct {
	DB *sql.DB
	S3 *s3client.Client
}

func (h *UploadHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/init", h.initUpload)
	r.Post("/complete", h.completeUpload)
	return r
}

func (h *UploadHandler) initUpload(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())

	var req models.UploadInitRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.Filename == "" {
		writeError(w, http.StatusBadRequest, "filename is required")
		return
	}
	if len(req.Filename) > 255 {
		writeError(w, http.StatusBadRequest, "filename too long (max 255)")
		return
	}
	if req.ContentType == "" {
		req.ContentType = "application/octet-stream"
	}

	fileID := uuid.New().String()
	s3Key := s3client.BuildS3Key(userID, req.Filename)

	record := &models.FileRecord{
		ID:               fileID,
		UserID:           userID,
		S3Key:            s3Key,
		OriginalFilename: req.Filename,
		ContentType:      req.ContentType,
		SizeBytes:        req.SizeBytes,
		SHA256:           req.SHA256,
		CreatedAt:        time.Now().UTC(),
		Status:           models.StatusInitiated,
	}

	if err := db.InsertFile(h.DB, record); err != nil {
		slog.Error("insert file record", "err", err, "user_id", userID, "file_id", fileID)
		writeError(w, http.StatusInternalServerError, "failed to create upload record")
		return
	}

	presignedURL, err := h.S3.GenerateUploadURL(r.Context(), s3Key, req.ContentType)
	if err != nil {
		slog.Error("generate presigned PUT URL", "err", err, "user_id", userID, "file_id", fileID)
		writeError(w, http.StatusBadGateway, "failed to generate upload URL")
		return
	}

	slog.Info("upload initiated", "user_id", userID, "file_id", fileID, "filename", req.Filename)

	writeJSON(w, http.StatusOK, models.UploadInitResponse{
		FileID:           fileID,
		S3Key:            s3Key,
		PresignedPutURL:  presignedURL,
		ExpiresInSeconds: h.S3.TTL(),
		RequiredHeaders: map[string]string{
			"Content-Type": req.ContentType,
		},
	})
}

func (h *UploadHandler) completeUpload(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())

	var req models.UploadCompleteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.FileID == "" {
		writeError(w, http.StatusBadRequest, "fileId is required")
		return
	}

	record, err := db.GetFileByIDAndUser(h.DB, req.FileID, userID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if err != nil {
		slog.Error("get file record", "err", err, "user_id", userID, "file_id", req.FileID)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	if record.Status != models.StatusInitiated {
		writeError(w, http.StatusConflict, "upload already completed or failed")
		return
	}

	var tagsJSON *string
	if req.Tags != nil {
		b, _ := json.Marshal(req.Tags)
		s := string(b)
		tagsJSON = &s
	}

	if err := db.UpdateFileStatus(h.DB, req.FileID, models.StatusUploaded, req.SizeBytes, req.SHA256, tagsJSON); err != nil {
		slog.Error("update file status", "err", err, "user_id", userID, "file_id", req.FileID)
		writeError(w, http.StatusInternalServerError, "failed to update upload status")
		return
	}

	// Re-fetch to return updated record.
	updated, err := db.GetFileByIDAndUser(h.DB, req.FileID, userID)
	if err != nil {
		slog.Error("re-fetch file record", "err", err, "user_id", userID, "file_id", req.FileID)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	slog.Info("upload completed", "user_id", userID, "file_id", req.FileID)
	writeJSON(w, http.StatusOK, models.FileRecordToResponse(updated))
}
