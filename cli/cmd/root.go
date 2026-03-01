package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pushkit/cli/internal/client"
	"github.com/pushkit/cli/internal/config"
)

var (
	flagAPIURL string
	flagAPIKey string
)

var rootCmd = &cobra.Command{
	Use:   "push",
	Short: "S3 Push CLI — upload and manage files",
	Long:  "A CLI tool for uploading files to S3 via a backend broker, and listing/downloading them.",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagAPIURL, "api-url", "", "API base URL (overrides config)")
	rootCmd.PersistentFlags().StringVar(&flagAPIKey, "api-key", "", "API key (overrides config)")
}

func getClient() (*client.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	apiURL := cfg.APIURL
	apiKey := cfg.APIKey

	if flagAPIURL != "" {
		apiURL = flagAPIURL
	}
	if flagAPIKey != "" {
		apiKey = flagAPIKey
	}

	if apiURL == "" {
		return nil, fmt.Errorf("API URL not configured. Run: push config set --api-url=<url> --api-key=<key>")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("API key not configured. Run: push config set --api-url=<url> --api-key=<key>")
	}

	return client.New(apiURL, apiKey), nil
}

func exitErr(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+msg+"\n", args...)
	os.Exit(1)
}
