package main

import (
	"os"

	"github.com/pushkit/cli/cmd"
)

// Version is set at build time via -ldflags "-X main.Version=...".
var Version = "dev"

func main() {
	cmd.SetVersion(Version)

	// cmd.Execute() handles all error output (JSON or plain) itself.
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
