package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pushkit/cli/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
}

var configSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set configuration values",
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
		fmt.Printf("Config saved to %s\n", p)
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		fmt.Printf("API URL: %s\n", cfg.APIURL)
		if len(cfg.APIKey) > 4 {
			fmt.Printf("API Key: ***%s\n", cfg.APIKey[len(cfg.APIKey)-4:])
		} else if cfg.APIKey != "" {
			fmt.Println("API Key: ****")
		} else {
			fmt.Println("API Key: (not set)")
		}

		p, _ := config.Path()
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
