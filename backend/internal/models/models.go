package models

import "time"

type FileStatus string

const (
	StatusInitiated FileStatus = "INITIATED"
	StatusUploaded  FileStatus = "UPLOADED"
	StatusFailed    FileStatus = "FAILED"
)

type FileRecord struct {
	ID               string     `json:"id"`
	UserID           string     `json:"user_id"`
	S3Key            string     `json:"s3_key"`
	OriginalFilename string     `json:"original_filename"`
	ContentType      string     `json:"content_type"`
	SizeBytes        *int64     `json:"size_bytes"`
	SHA256           *string    `json:"sha256"`
	TagsJSON         *string    `json:"tags_json,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	Status           FileStatus `json:"status"`
}

// API request/response types

type UploadInitRequest struct {
	Filename    string  `json:"filename"`
	ContentType string  `json:"contentType"`
	SizeBytes   *int64  `json:"sizeBytes,omitempty"`
	SHA256      *string `json:"sha256,omitempty"`
}

type UploadInitResponse struct {
	FileID           string            `json:"fileId"`
	S3Key            string            `json:"s3Key"`
	PresignedPutURL  string            `json:"presignedPutUrl"`
	ExpiresInSeconds int               `json:"expiresInSeconds"`
	RequiredHeaders  map[string]string `json:"requiredHeaders"`
}

type UploadCompleteRequest struct {
	FileID    string            `json:"fileId"`
	SizeBytes *int64            `json:"sizeBytes,omitempty"`
	SHA256    *string           `json:"sha256,omitempty"`
	Tags      map[string]string `json:"tags,omitempty"`
}

type FileResponse struct {
	ID               string `json:"id"`
	OriginalFilename string `json:"originalFilename"`
	ContentType      string `json:"contentType"`
	SizeBytes        *int64 `json:"sizeBytes"`
	CreatedAt        string `json:"createdAt"`
	Status           string `json:"status"`
}

type FileListResponse struct {
	Items      []FileResponse `json:"items"`
	NextCursor *string        `json:"nextCursor"`
}

type DownloadResponse struct {
	PresignedGetURL  string `json:"presignedGetUrl"`
	ExpiresInSeconds int    `json:"expiresInSeconds"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func FileRecordToResponse(f *FileRecord) FileResponse {
	return FileResponse{
		ID:               f.ID,
		OriginalFilename: f.OriginalFilename,
		ContentType:      f.ContentType,
		SizeBytes:        f.SizeBytes,
		CreatedAt:        f.CreatedAt.UTC().Format(time.RFC3339),
		Status:           string(f.Status),
	}
}
