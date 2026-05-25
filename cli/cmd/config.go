package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pushkit/cli/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
	Long: `View and update PushKit CLI configuration.

Configuration is stored in a JSON file at a platform-specific location:
  macOS:   ~/Library/Application Support/s3push/config.json
  Linux:   $XDG_CONFIG_HOME/s3push/config.json (or ~/.config/s3push/)
  Windows: %APPDATA%\s3push\config.json

Subcommands:
  set    Save API URL and/or API key
  show   Display the current configuration`,
}

var configSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set configuration values",
	Long: `Save API URL and/or API key to the config file.

At least one of --api-url or --api-key should be provided.
Existing values are preserved if not overridden.

JSON output (--json):
  {"saved":true,"configPath":"..."}`,
	Example: `  # Set both API URL and key
  pushkit config set --api-url=https://api.example.com --api-key=sk-abc123

  # Update just the API key
  pushkit config set --api-key=sk-newkey

  # Set config with JSON confirmation (for scripts/agents)
  pushkit config set --api-url=https://api.example.com --api-key=sk-abc123 --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		apiURL, _ := cmd.Flags().GetString("api-url")
		apiKey, _ := cmd.Flags().GetString("api-key")

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if apiURL != "" {
			cfg.APIURL = apiURL
		}
		if apiKey != "" {
			cfg.APIKey = apiKey
		}

		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		p, _ := config.Path()

		if flagJSON {
			return outputJSON(struct {
				Saved      bool   `json:"saved"`
				ConfigPath string `json:"configPath"`
			}{Saved: true, ConfigPath: p})
		}

		fmt.Printf("Config saved to %s\n", p)
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long: `Display the current CLI configuration.

Shows the API URL, a masked API key, and the config file path.

JSON output (--json):
  {"apiUrl":"...","apiKeySet":true,"configPath":"..."}
  Note: the full API key is never included in output for security.`,
	Example: `  # Show current config
  pushkit config show

  # Show config as JSON (for scripts/agents)
  pushkit config show --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		p, _ := config.Path()

		if flagJSON {
			return outputJSON(struct {
				APIURL     string `json:"apiUrl"`
				APIKeySet  bool   `json:"apiKeySet"`
				ConfigPath string `json:"configPath"`
			}{
				APIURL:     cfg.APIURL,
				APIKeySet:  cfg.APIKey != "",
				ConfigPath: p,
			})
		}

		fmt.Printf("API URL: %s\n", cfg.APIURL)
		if len(cfg.APIKey) > 4 {
			fmt.Printf("API Key: ***%s\n", cfg.APIKey[len(cfg.APIKey)-4:])
		} else if cfg.APIKey != "" {
			fmt.Println("API Key: ****")
		} else {
			fmt.Println("API Key: (not set)")
		}

		fmt.Printf("Config file: %s\n", p)
		return nil
	},
}

func init() {
	configSetCmd.Flags().String("api-url", "", "API base URL")
	configSetCmd.Flags().String("api-key", "", "API key")

	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configShowCmd)
	rootCmd.AddCommand(configCmd)
}
