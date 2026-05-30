package main

import (
	"os"

	"github.com/pushkit/cli/cmd"
)

// Version is set at build time via -ldflags "-X main.Version=...".
var Version = "dev"

// Version output format: "pushkit version <version>" (Cobra default template).
// This intentionally differs from the server binary ("pushkit-server <version>")
// which uses stdlib flag to stay dependency-light. The divergence is documented;
// do not unify unless the added Cobra dependency on the server side is acceptable.

func main() {
	cmd.SetVersion(Version)

	// cmd.Execute() handles all error output (JSON or plain) itself.
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
