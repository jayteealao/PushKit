package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/pushkit/cli/internal/progress"
)

var downloadCmd = &cobra.Command{
	Use:   "download <fileId>",
	Short: "Download a file",
	Long: `Download a file by its ID.

The download flow:
  1. Request a presigned GET URL from the API
  2. Stream the file from S3 to a local file

The output filename defaults to the original filename (from Content-Disposition)
or the file ID if unavailable. Use --out to specify a custom path.

JSON output (--json):
  On success, prints a JSON object to stdout:
    {"fileId":"...","filename":"...","path":"/abs/path","sizeBytes":1234}
  On failure, prints {"error":"message"} to stderr and exits non-zero.
  Progress bars and status messages are suppressed.`,
	Example: `  # Download a file
  pushkit download abc123

  # Download to a specific path, overwriting if exists
  pushkit download abc123 --out ./local-copy.pdf --force

  # Download with JSON output (for scripts/agents)
  pushkit download abc123 --json`,
	Args: cobra.ExactArgs(1),
	RunE: runDownload,
}

var (
	downloadOutput string
	downloadForce  bool
)

func init() {
	downloadCmd.Flags().StringVarP(&downloadOutput, "out", "o", "", "Output file path (default: original filename in current dir)")
	downloadCmd.Flags().BoolVarP(&downloadForce, "force", "f", false, "Overwrite existing file")
	rootCmd.AddCommand(downloadCmd)
}

func runDownload(cmd *cobra.Command, args []string) error {
	fileID := args[0]

	c, err := getClient()
	if err != nil {
		return err
	}

	ctx := context.Background()

	logStderr("Getting download URL...\n")
	resp, err := c.GetDownloadURL(ctx, fileID)
	if err != nil {
		return fmt.Errorf("get download URL: %w", err)
	}

	// Download from presigned URL using a client with a generous timeout for large files.
	dlClient := &http.Client{Timeout: 10 * time.Minute}
	dlReq, err := http.NewRequestWithContext(ctx, http.MethodGet, resp.PresignedGetURL, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	httpResp, err := dlClient.Do(dlReq)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", httpResp.StatusCode)
	}

	outPath := downloadOutput
	if outPath == "" {
		// Try to extract filename from Content-Disposition or use fileID.
		outPath = fileID
		if cd := httpResp.Header.Get("Content-Disposition"); cd != "" {
			if _, params, err := parseContentDisposition(cd); err == nil {
				if fn, ok := params["filename"]; ok && fn != "" {
					outPath = fn
				}
			}
		}
	}

	outPath, err = filepath.Abs(outPath)
	if err != nil {
		return fmt.Errorf("resolve output path: %w", err)
	}

	if !downloadForce {
		if _, err := os.Stat(outPath); err == nil {
			return fmt.Errorf("file %s already exists (use --force to overwrite)", outPath)
		}
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}

	size := httpResp.ContentLength

	// In JSON mode, skip the progress bar — read directly from the response.
	var body io.Reader = httpResp.Body
	var pr *progress.Reader
	if !flagJSON {
		pr = progress.NewReader(httpResp.Body, size)
		body = pr
	}

	logStderr("Downloading...\n")
	n, err := io.Copy(f, body)
	if closeErr := f.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(outPath) // Clean up partial file.
		return fmt.Errorf("download failed: %w", err)
	}
	if pr != nil {
		pr.Finish()
	}

	if flagJSON {
		return outputJSON(struct {
			FileID    string `json:"fileId"`
			Filename  string `json:"filename"`
			Path      string `json:"path"`
			SizeBytes int64  `json:"sizeBytes"`
		}{
			FileID:    fileID,
			Filename:  filepath.Base(outPath),
			Path:      outPath,
			SizeBytes: n,
		})
	}

	fmt.Printf("Downloaded %s (%s) to %s\n",
		filepath.Base(outPath), progress.FormatBytes(n), outPath)
	return nil
}

// parseContentDisposition is a simplified parser for Content-Disposition header.
func parseContentDisposition(header string) (string, map[string]string, error) {
	params := make(map[string]string)
	// Simple parsing: look for filename="..."
	for _, part := range splitParams(header) {
		if len(part) > 10 && part[:9] == "filename=" {
			val := part[9:]
			val = trimQuotes(val)
			params["filename"] = val
		}
	}
	return "attachment", params, nil
}

func splitParams(s string) []string {
	var parts []string
	for _, p := range splitSemicolon(s) {
		p = trimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func splitSemicolon(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ';' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSpace(s string) string {
	for len(s) > 0 && s[0] == ' ' {
		s = s[1:]
	}
	for len(s) > 0 && s[len(s)-1] == ' ' {
		s = s[:len(s)-1]
	}
	return s
}

func trimQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
