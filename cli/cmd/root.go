package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pushkit/cli/internal/client"
	"github.com/pushkit/cli/internal/config"
)

var (
	flagAPIURL string
	flagAPIKey string
	flagJSON   bool
)

var rootCmd = &cobra.Command{
	Use:   "pushkit",
	Short: "PushKit CLI — upload and manage files via S3",
	Long: `PushKit CLI — upload, list, and download files via an S3-backed API.

Workflow:
  1. Configure credentials:  pushkit config set --api-url=<url> --api-key=<key>
  2. Upload a file:          pushkit upload <file>
  3. List files:             pushkit ls
  4. Download a file:        pushkit download <fileId>

Upload flow (3 steps handled automatically):
  init → presigned S3 PUT → complete

Structured output for agents:
  Pass --json to any command to get machine-readable JSON on stdout.
  Errors become {"error":"message"} on stderr with a non-zero exit code.
  Progress bars and informational messages are suppressed in JSON mode.

Environment variables:
  PUSHKIT_API_URL   API base URL
  PUSHKIT_API_KEY   API key

  Precedence: --flags > env vars > config file.

Global flags:
  --json         Output structured JSON (for scripts and AI agents)
  --api-url      Override the configured API base URL
  --api-key      Override the configured API key`,
}

// SetVersion sets the version string shown by --version.
func SetVersion(v string) {
	rootCmd.Version = v
}

// Execute runs the root command. Exported for main.go.
func Execute() error {
	return rootCmd.Execute()
}

// IsJSON reports whether the --json flag is set.
func IsJSON() bool {
	return flagJSON
}

// outputJSON marshals v to stdout as indented JSON.
func outputJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// outputError prints an error. When --json is set, it emits {"error":"..."} to stderr.
func outputError(err error) {
	if flagJSON {
		b, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintln(os.Stderr, string(b))
	} else {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagAPIURL, "api-url", "", "API base URL (overrides config)")
	rootCmd.PersistentFlags().StringVar(&flagAPIKey, "api-key", "", "API key (overrides config)")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output structured JSON (for scripts and AI agents)")
}

func getClient() (*client.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Precedence: CLI flags > env vars > config file.
	apiURL := cfg.APIURL
	apiKey := cfg.APIKey

	if v := os.Getenv("PUSHKIT_API_URL"); v != "" {
		apiURL = v
	}
	if v := os.Getenv("PUSHKIT_API_KEY"); v != "" {
		apiKey = v
	}

	if flagAPIURL != "" {
		apiURL = flagAPIURL
	}
	if flagAPIKey != "" {
		apiKey = flagAPIKey
	}

	if apiURL == "" {
		return nil, fmt.Errorf("API URL not configured. Set PUSHKIT_API_URL or run: pushkit config set --api-url=<url>")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("API key not configured. Set PUSHKIT_API_KEY or run: pushkit config set --api-key=<key>")
	}

	return client.New(apiURL, apiKey), nil
}

// logStderr prints a message to stderr, suppressed in JSON mode.
func logStderr(format string, args ...any) {
	if !flagJSON {
		fmt.Fprintf(os.Stderr, format, args...)
	}
}
