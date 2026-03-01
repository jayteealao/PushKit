package db

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/pushkit/backend/internal/models"
)

func InsertFile(db *sql.DB, f *models.FileRecord) error {
	_, err := db.Exec(
		`INSERT INTO files (id, user_id, s3_key, original_filename, content_type, size_bytes, sha256, tags_json, created_at, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.UserID, f.S3Key, f.OriginalFilename, f.ContentType,
		f.SizeBytes, f.SHA256, f.TagsJSON, f.CreatedAt.UTC().Format(time.RFC3339), f.Status,
	)
	if err != nil {
		return fmt.Errorf("insert file: %w", err)
	}
	return nil
}

func GetFileByIDAndUser(db *sql.DB, fileID, userID string) (*models.FileRecord, error) {
	row := db.QueryRow(
		`SELECT id, user_id, s3_key, original_filename, content_type, size_bytes, sha256, tags_json, created_at, status
		 FROM files WHERE id = ? AND user_id = ?`,
		fileID, userID,
	)
	return scanFileRecord(row)
}

func GetFileByID(db *sql.DB, fileID string) (*models.FileRecord, error) {
	row := db.QueryRow(
		`SELECT id, user_id, s3_key, original_filename, content_type, size_bytes, sha256, tags_json, created_at, status
		 FROM files WHERE id = ?`,
		fileID,
	)
	return scanFileRecord(row)
}

func UpdateFileStatus(db *sql.DB, fileID string, status models.FileStatus, sizeBytes *int64, sha256 *string, tagsJSON *string) error {
	_, err := db.Exec(
		`UPDATE files SET status = ?, size_bytes = COALESCE(?, size_bytes), sha256 = COALESCE(?, sha256), tags_json = COALESCE(?, tags_json) WHERE id = ?`,
		status, sizeBytes, sha256, tagsJSON, fileID,
	)
	if err != nil {
		return fmt.Errorf("update file status: %w", err)
	}
	return nil
}

func DeleteFile(db *sql.DB, fileID string) error {
	res, err := db.Exec(`DELETE FROM files WHERE id = ?`, fileID)
	if err != nil {
		return fmt.Errorf("delete file: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

type ListParams struct {
	UserID string
	Cursor string
	Limit  int
	Search string
	Sort   string
	Order  string
}

func ListFiles(database *sql.DB, p ListParams) ([]models.FileRecord, *string, error) {
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 20
	}
	if p.Sort == "" {
		p.Sort = "created_at"
	}
	if p.Order == "" {
		p.Order = "desc"
	}

	// Validate sort/order to prevent injection.
	allowedSort := map[string]bool{"created_at": true, "original_filename": true, "size_bytes": true}
	if !allowedSort[p.Sort] {
		p.Sort = "created_at"
	}
	if p.Order != "asc" && p.Order != "desc" {
		p.Order = "desc"
	}

	var conditions []string
	var args []any

	conditions = append(conditions, "user_id = ?")
	args = append(args, p.UserID)

	conditions = append(conditions, "status = ?")
	args = append(args, string(models.StatusUploaded))

	if p.Search != "" {
		// Escape LIKE wildcards in user input.
		escaped := strings.ReplaceAll(p.Search, "%", "\\%")
		escaped = strings.ReplaceAll(escaped, "_", "\\_")
		conditions = append(conditions, "original_filename LIKE ? ESCAPE '\\'")
		args = append(args, "%"+escaped+"%")
	}

	if p.Cursor != "" {
		cursorTS, cursorID, err := decodeCursor(p.Cursor)
		if err == nil {
			op := "<"
			if p.Order == "asc" {
				op = ">"
			}
			conditions = append(conditions, fmt.Sprintf("(created_at, id) %s (?, ?)", op))
			args = append(args, cursorTS, cursorID)
		}
	}

	where := "WHERE " + strings.Join(conditions, " AND ")
	// Sort and order are validated above, safe to interpolate.
	orderClause := fmt.Sprintf("ORDER BY %s %s, id %s", p.Sort, p.Order, p.Order)
	query := fmt.Sprintf(
		"SELECT id, user_id, s3_key, original_filename, content_type, size_bytes, sha256, tags_json, created_at, status FROM files %s %s LIMIT ?",
		where, orderClause,
	)
	args = append(args, p.Limit+1)

	rows, err := database.Query(query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("list files: %w", err)
	}
	defer rows.Close()

	var files []models.FileRecord
	for rows.Next() {
		f, err := scanFileRecordFromRows(rows)
		if err != nil {
			return nil, nil, err
		}
		files = append(files, *f)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate rows: %w", err)
	}

	var nextCursor *string
	if len(files) > p.Limit {
		files = files[:p.Limit]
		last := files[len(files)-1]
		c := encodeCursor(last.CreatedAt.UTC().Format(time.RFC3339), last.ID)
		nextCursor = &c
	}

	return files, nextCursor, nil
}

func encodeCursor(ts, id string) string {
	return base64.URLEncoding.EncodeToString([]byte(ts + "|" + id))
}

func decodeCursor(cursor string) (string, string, error) {
	data, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return "", "", err
	}
	parts := strings.SplitN(string(data), "|", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid cursor format")
	}
	return parts[0], parts[1], nil
}

func scanFileRecord(row *sql.Row) (*models.FileRecord, error) {
	var f models.FileRecord
	var createdAtStr string
	err := row.Scan(
		&f.ID, &f.UserID, &f.S3Key, &f.OriginalFilename, &f.ContentType,
		&f.SizeBytes, &f.SHA256, &f.TagsJSON, &createdAtStr, &f.Status,
	)
	if err != nil {
		return nil, err
	}
	f.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	return &f, nil
}

func scanFileRecordFromRows(rows *sql.Rows) (*models.FileRecord, error) {
	var f models.FileRecord
	var createdAtStr string
	err := rows.Scan(
		&f.ID, &f.UserID, &f.S3Key, &f.OriginalFilename, &f.ContentType,
		&f.SizeBytes, &f.SHA256, &f.TagsJSON, &createdAtStr, &f.Status,
	)
	if err != nil {
		return nil, err
	}
	f.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	return &f, nil
}
