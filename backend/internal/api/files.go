package api

import (
	"database/sql"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/pushkit/backend/internal/auth"
	"github.com/pushkit/backend/internal/db"
	"github.com/pushkit/backend/internal/models"
	s3client "github.com/pushkit/backend/internal/s3"
)

type FileHandler struct {
	DB *sql.DB
	S3 *s3client.Client
}

func (h *FileHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.listFiles)
	r.Get("/{fileId}/download", h.downloadFile)
	r.Delete("/{fileId}", h.deleteFile)
	return r
}

func (h *FileHandler) listFiles(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	q := r.URL.Query()

	limit := 20
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n >= 1 && n <= 100 {
			limit = n
		}
	}

	params := db.ListParams{
		UserID: userID,
		Cursor: q.Get("cursor"),
		Limit:  limit,
		Search: q.Get("q"),
		Sort:   q.Get("sort"),
		Order:  q.Get("order"),
	}

	files, nextCursor, err := db.ListFiles(h.DB, params)
	if err != nil {
		slog.Error("list files", "err", err, "user_id", userID)
		writeError(w, http.StatusInternalServerError, "failed to list files")
		return
	}

	items := make([]models.FileResponse, 0, len(files))
	for i := range files {
		items = append(items, models.FileRecordToResponse(&files[i]))
	}

	writeJSON(w, http.StatusOK, models.FileListResponse{
		Items:      items,
		NextCursor: nextCursor,
	})
}

func (h *FileHandler) downloadFile(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	fileID := chi.URLParam(r, "fileId")

	record, err := db.GetFileByIDAndUser(h.DB, fileID, userID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if err != nil {
		slog.Error("get file for download", "err", err, "user_id", userID, "file_id", fileID)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	if record.Status != models.StatusUploaded {
		writeError(w, http.StatusNotFound, "file not available for download")
		return
	}

	presignedURL, err := h.S3.GenerateDownloadURL(r.Context(), record.S3Key, record.OriginalFilename)
	if err != nil {
		slog.Error("generate presigned GET URL", "err", err, "user_id", userID, "file_id", fileID)
		writeError(w, http.StatusBadGateway, "failed to generate download URL")
		return
	}

	slog.Info("download requested", "user_id", userID, "file_id", fileID)
	writeJSON(w, http.StatusOK, models.DownloadResponse{
		PresignedGetURL:  presignedURL,
		ExpiresInSeconds: h.S3.TTL(),
	})
}

func (h *FileHandler) deleteFile(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	fileID := chi.URLParam(r, "fileId")

	record, err := db.GetFileByIDAndUser(h.DB, fileID, userID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if err != nil {
		slog.Error("get file for delete", "err", err, "user_id", userID, "file_id", fileID)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Best-effort S3 deletion: log errors but still delete DB record.
	if err := h.S3.DeleteObject(r.Context(), record.S3Key); err != nil {
		slog.Warn("failed to delete S3 object", "err", err, "s3_key", record.S3Key, "file_id", fileID)
	}

	if err := db.DeleteFile(h.DB, fileID); err != nil {
		slog.Error("delete file record", "err", err, "user_id", userID, "file_id", fileID)
		writeError(w, http.StatusInternalServerError, "failed to delete file")
		return
	}

	slog.Info("file deleted", "user_id", userID, "file_id", fileID)
	w.WriteHeader(http.StatusNoContent)
}
