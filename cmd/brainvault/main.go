// Command brainvault serves a Markdown vault to agents as MCP tools, and offers the same operations as a CLI.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/poom-h/brainvault/internal/audit"
	"github.com/poom-h/brainvault/internal/mcpserver"
	"github.com/poom-h/brainvault/internal/vault"
)

// version is set at release time with -ldflags "-X main.version=…".
var version = "dev"

const usage = `brainvault: the Memory Vault as a tool

Usage:
  brainvault mcp                                      run the MCP server over stdio
  brainvault search [--folder F] [--limit N] QUERY
  brainvault read ID
  brainvault list [--folder F] [--flat]
  brainvault backlinks ID
  brainvault write [--version V] ID < content
  brainvault doctor                                   check the vault; exits 1 on errors
  brainvault version

Every command takes --vault (default $BRAINVAULT_PATH) and --actor (default $BRAINVAULT_ACTOR).
Put flags before the note ID or query.`

var commands = map[string]bool{"mcp": true, "search": true, "read": true, "list": true, "backlinks": true, "write": true, "doctor": true}

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "help", "-h", "--help":
		fmt.Fprintln(stdout, usage)
		return 0
	case "version":
		fmt.Fprintln(stdout, version)
		return 0
	}
	if !commands[cmd] {
		fmt.Fprintf(stderr, "brainvault: unknown command %q\n\n%s\n", cmd, usage)
		return 2
	}

	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	vaultPath := fs.String("vault", os.Getenv("BRAINVAULT_PATH"), "vault folder (default $BRAINVAULT_PATH)")
	actor := fs.String("actor", envOr("BRAINVAULT_ACTOR", "unknown"), "caller name recorded in the log (default $BRAINVAULT_ACTOR)")
	folder := fs.String("folder", "", "limit to this vault folder (search, list)")
	limit := fs.Int("limit", 20, "maximum results (search)")
	flat := fs.Bool("flat", false, "only notes directly in the folder (list)")
	noteVersion := fs.String("version", "", "version from your last read, to replace a note (write)")
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
	log := audit.New(v.Root)

	if cmd == "mcp" {
		srv := mcpserver.New(v, log, *actor, version)
		if err := srv.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			fmt.Fprintln(stderr, "brainvault:", err)
			return 1
		}
		return 0
	}

	c := cli{vault: v, log: log, actor: *actor, out: stdout}
	switch cmd {
	case "search":
		err = c.search(strings.Join(fs.Args(), " "), *folder, *limit)
	case "read":
		err = c.read(fs.Args())
	case "list":
		err = c.list(*folder, !*flat)
	case "backlinks":
		err = c.backlinks(fs.Args())
	case "write":
		err = c.write(fs.Args(), *noteVersion, stdin)
	case "doctor":
		var clean bool
		clean, err = c.doctor()
		if err == nil && !clean {
			return 1
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, "brainvault:", err)
		return 1
	}
	return 0
}
