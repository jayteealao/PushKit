package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/pushkit/cli/internal/client"
	"github.com/pushkit/cli/internal/config"
	mcpserver "github.com/pushkit/cli/internal/mcp"
)

var mcpCmd = &cobra.Command{
	Use:    "mcp",
	Short:  "Run a stdio MCP server exposing PushKit file tools",
	Hidden: true,
	Long: `Run a Model Context Protocol (MCP) server over stdio that exposes PushKit
file tools (pushkit_push, pushkit_pull, pushkit_list, pushkit_delete) to an MCP
client such as Claude Code.

This command is normally launched by the MCP client, not run by hand. Register it
with the committed .mcp.json template or with:

  claude mcp add pushkit -- pushkit mcp

Credentials come from the environment (PUSHKIT_API_KEY, PUSHKIT_API_URL) or the
stored config. The server starts even when credentials are missing or the backend
is unreachable; each tool call then returns a clear error.`,
	// Cobra's PersistentPreRunE (the --api-key process-list warning) would print
	// to stderr, which is fine for stdio MCP, but the server should otherwise stay
	// silent on stdout. All diagnostics go to stderr.
	RunE: runMCP,
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}

func runMCP(cmd *cobra.Command, _ []string) error {
	// Start-and-error: resolve credentials but never refuse to start. A missing
	// key/URL or unreachable backend surfaces as a per-tool error, which is the
	// most debuggable behavior for an agent.
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "pushkit mcp: ignoring config load error: %v\n", err)
		cfg = &config.Config{}
	}
	apiURL, apiKey := resolveCredentials(
		cfg.APIURL, cfg.APIKey,
		os.Getenv("PUSHKIT_API_URL"), os.Getenv("PUSHKIT_API_KEY"),
		flagAPIURL, flagAPIKey,
	)

	srv := mcpserver.NewForClient(client.New(apiURL, apiKey), apiKey, apiURL, rootCmd.Version)

	// A stdio MCP server ends when the client closes the connection (stdin EOF)
	// or the context is cancelled — both are normal shutdowns, not errors. Only a
	// genuine transport/protocol failure should produce a non-zero exit.
	if err := srv.Run(cmd.Context(), &mcp.StdioTransport{}); !isCleanShutdown(err) {
		return err
	}
	return nil
}

// isCleanShutdown reports whether an error from the stdio server's Run is a
// normal end-of-session (the client closed stdin, or the context was cancelled)
// rather than a real failure. A clean disconnect surfaces as io.EOF/Canceled, or
// — once a request has been processed — as the SDK's internal jsonrpc2
// "server is closing" wire error, which lives in an internal package (no
// exported sentinel) and does not Unwrap to io.EOF, so it is matched by message.
func isCleanShutdown(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "server is closing") ||
		strings.Contains(msg, "client is closing") ||
		strings.HasSuffix(msg, "EOF")
}
