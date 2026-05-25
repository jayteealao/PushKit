package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	apiclient "github.com/pushkit/cli/internal/client"
	"github.com/pushkit/cli/internal/progress"
)

var uploadCmd = &cobra.Command{
	Use:   "upload <file>",
	Short: "Upload a file to S3",
	Long: `Upload a file to S3 via the PushKit API.

The upload is a 3-step process handled automatically:
  1. init    — reserve a file ID and get a presigned S3 PUT URL
  2. PUT     — upload the file bytes directly to S3
  3. complete — finalize the upload and record metadata

Options:
  --name     Override the filename stored in the API (default: basename of <file>)
  --tag      Attach key=value metadata tags (repeatable)
  --sha256   Compute and send a SHA-256 hash for server-side integrity verification

JSON output (--json):
  On success, prints a JSON object to stdout:
    {"id":"...","originalFilename":"...","contentType":"...","sizeBytes":1234,"status":"uploaded"}
  On failure, prints {"error":"message"} to stderr and exits non-zero.
  Progress bars and status messages are suppressed.`,
	Example: `  # Upload a file (human-readable output)
  pushkit upload ./report.pdf

  # Upload with custom name and tags
  pushkit upload ./data.csv --name quarterly.csv --tag team=analytics --tag quarter=Q1

  # Upload with integrity check and JSON output (for scripts/agents)
  pushkit upload ./backup.tar.gz --sha256 --json`,
	Args: cobra.ExactArgs(1),
	RunE: runUpload,
}

var (
	uploadName   string
	uploadTags   []string
	uploadSHA256 bool
)

func init() {
	uploadCmd.Flags().StringVar(&uploadName, "name", "", "Custom filename (default: original name)")
	uploadCmd.Flags().StringSliceVar(&uploadTags, "tag", nil, "Tags in key=value format (repeatable)")
	uploadCmd.Flags().BoolVar(&uploadSHA256, "sha256", false, "Compute and send SHA-256 hash")
	rootCmd.AddCommand(uploadCmd)
}

func runUpload(cmd *cobra.Command, args []string) error {
	filePath := args[0]

	fi, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("cannot access %s: %w", filePath, err)
	}
	if fi.IsDir() {
		return fmt.Errorf("%s is a directory, not a file", filePath)
	}

	filename := fi.Name()
	if uploadName != "" {
		filename = uploadName
	}

	contentType := detectContentType(filename)
	size := fi.Size()

	var hashStr *string
	if uploadSHA256 {
		logStderr("Computing SHA-256...\n")
		h, err := computeSHA256(filePath)
		if err != nil {
			return fmt.Errorf("compute SHA-256: %w", err)
		}
		hashStr = &h
		logStderr("SHA-256: %s\n", h)
	}

	tags := parseTags(uploadTags)

	c, err := getClient()
	if err != nil {
		return err
	}

	ctx := context.Background()

	logStderr("Initializing upload for %s (%s, %s)...\n",
		filename, contentType, progress.FormatBytes(size))

	initResp, err := c.InitUpload(ctx, &apiclient.UploadInitRequest{
		Filename:    filename,
		ContentType: contentType,
		SizeBytes:   size,
		SHA256:      hashStr,
	})
	if err != nil {
		return fmt.Errorf("init upload: %w", err)
	}

	logStderr("File ID: %s\n", initResp.FileID)

	// Use Content-Type from the backend's requiredHeaders (which matches the
	// presigned URL signature) rather than the locally-detected value.
	putContentType := contentType
	if ct, ok := initResp.RequiredHeaders["Content-Type"]; ok && ct != "" {
		putContentType = ct
	}

	logStderr("Uploading to S3...\n")

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	// In JSON mode, skip the progress bar — pass the raw file reader.
	var body io.Reader = f
	var pr *progress.Reader
	if !flagJSON {
		pr = progress.NewReader(f, size)
		body = pr
	}

	if err := c.PutToPresignedURL(ctx, initResp.PresignedPutURL, body, putContentType, size); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}
	if pr != nil {
		pr.Finish()
	}

	logStderr("Finalizing upload...\n")

	completeReq := &apiclient.UploadCompleteRequest{
		FileID:    initResp.FileID,
		SizeBytes: size,
		SHA256:    hashStr,
		Tags:      tags,
	}

	result, err := c.CompleteUpload(ctx, completeReq)
	if err != nil {
		return fmt.Errorf("complete upload: %w", err)
	}

	if flagJSON {
		return outputJSON(result)
	}

	fmt.Printf("Upload complete!\n")
	fmt.Printf("  File ID:  %s\n", result.ID)
	fmt.Printf("  Filename: %s\n", result.OriginalFilename)
	fmt.Printf("  Status:   %s\n", result.Status)
	return nil
}

func detectContentType(filename string) string {
	ext := filepath.Ext(filename)
	if ext != "" {
		ct := mime.TypeByExtension(ext)
		if ct != "" {
			return ct
		}
	}
	return "application/octet-stream"
}

func computeSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func parseTags(raw []string) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	tags := make(map[string]string)
	for _, t := range raw {
		parts := strings.SplitN(t, "=", 2)
		if len(parts) == 2 {
			tags[parts[0]] = parts[1]
		}
	}
	if len(tags) == 0 {
		return nil
	}
	return tags
}
