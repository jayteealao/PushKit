package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pushkit/cli/cmd"
)

// Version is set at build time via -ldflags "-X main.Version=...".
var Version = "dev"

func main() {
	cmd.SetVersion(Version)

	if err := cmd.Execute(); err != nil {
		if cmd.IsJSON() {
			b, _ := json.Marshal(map[string]string{"error": err.Error()})
			fmt.Fprintln(os.Stderr, string(b))
		}
		// Cobra already prints the error in non-JSON mode.
		os.Exit(1)
	}
}
