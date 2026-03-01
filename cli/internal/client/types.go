package client

type UploadInitRequest struct {
	Filename    string  `json:"filename"`
	ContentType string  `json:"contentType"`
	SizeBytes   int64   `json:"sizeBytes"`
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
	SizeBytes int64             `json:"sizeBytes"`
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
