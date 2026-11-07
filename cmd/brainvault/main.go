// Command brainvault serves a Markdown vault to agents as MCP tools.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/poom-h/brainvault/internal/audit"
	"github.com/poom-h/brainvault/internal/mcpserver"
	"github.com/poom-h/brainvault/internal/vault"
)

// version is set at release time with -ldflags "-X main.version=…".
var version = "dev"

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: brainvault mcp | version")
		return 2
	}
	cmd, rest := args[0], args[1:]
	if cmd == "version" {
		fmt.Fprintln(stdout, version)
		return 0
	}
	if cmd != "mcp" {
		fmt.Fprintf(stderr, "brainvault: unknown command %q\n", cmd)
		return 2
	}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	vaultPath := fs.String("vault", os.Getenv("BRAINVAULT_PATH"), "vault folder (default $BRAINVAULT_PATH)")
	actor := fs.String("actor", envOr("BRAINVAULT_ACTOR", "unknown"), "caller name recorded in the log (default $BRAINVAULT_ACTOR)")
	if err := fs.Parse(rest); err != nil {
		return 2
	}
	if *vaultPath == "" {
		fmt.Fprintln(stderr, "brainvault: no vault folder; set BRAINVAULT_PATH or pass --vault")
		return 1
	}
	v, err := vault.Open(*vaultPath)
	if err != nil {
		fmt.Fprintln(stderr, "brainvault:", err)
		return 1
	}
	srv := mcpserver.New(v, audit.New(v.Root), *actor, version)
	if err := srv.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintln(stderr, "brainvault:", err)
		return 1
	}
	return 0
}
