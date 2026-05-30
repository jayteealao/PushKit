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
	// PersistentPreRunE runs before every subcommand.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// M-10: warn when --api-key is supplied on the command line because it
		// exposes the key in the OS process list. Suppress in --json mode.
		if cmd.Root().PersistentFlags().Changed("api-key") {
			logStderr("Warning: --api-key is visible in the OS process list. " +
				"Prefer the PUSHKIT_API_KEY environment variable or the config file instead.\n")
		}
		return nil
	},
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

// Execute runs the root command and handles error output itself so callers
// only need to check for non-nil to decide the exit code.
func Execute() error {
	err := rootCmd.Execute()
	if err != nil {
		outputError(err)
	}
	return err
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
	// Silence Cobra's built-in error/usage printing so we control the single
	// error-output path (outputError) and avoid double-printing in --json mode.
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true

	rootCmd.PersistentFlags().StringVar(&flagAPIURL, "api-url", "", "API base URL (overrides config)")
	rootCmd.PersistentFlags().StringVar(&flagAPIKey, "api-key", "", "API key (overrides config)")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output structured JSON (for scripts and AI agents)")
}

// resolveCredentials returns the effective API URL and key by applying the
// precedence rule: config file < env vars < CLI flags. It is pure (no I/O).
func resolveCredentials(cfgURL, cfgKey, envURL, envKey, flagURL, flagKey string) (url, key string) {
	url, key = cfgURL, cfgKey
	if envURL != "" {
		url = envURL
	}
	if envKey != "" {
		key = envKey
	}
	if flagURL != "" {
		url = flagURL
	}
	if flagKey != "" {
		key = flagKey
	}
	return url, key
}

func getClient() (*client.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Precedence: CLI flags > env vars > config file.
	apiURL, apiKey := resolveCredentials(
		cfg.APIURL, cfg.APIKey,
		os.Getenv("PUSHKIT_API_URL"), os.Getenv("PUSHKIT_API_KEY"),
		flagAPIURL, flagAPIKey,
	)

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
