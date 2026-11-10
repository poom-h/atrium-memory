# atrium-memory

The Memory Vault as a tool. `atrium-memory` serves a folder of Markdown notes (an Obsidian-style vault) to AI agents as MCP tools, and has a command line over the same core: `search`, `read`, `list`, `backlinks`, `write` and `doctor`. It runs on macOS, Windows and Linux as a single binary.

## Install

Download the archive for your OS from the GitHub Releases page, unpack it, and put `atrium-memory` on your PATH.

Until releases are notarised, macOS may block a downloaded binary. Clear the quarantine flag once:

```sh
xattr -d com.apple.quarantine atrium-memory
```

Or, with Go installed:

```sh
go install github.com/poom-h/atrium-memory/cmd/atrium-memory@latest
```

## Tools

| Tool | What it does |
|---|---|
| `vault_search` | Find notes; every word must match the title, path or text |
| `vault_read` | A note's Markdown, properties, outgoing links and version |
| `vault_list` | Notes in the vault or one folder |
| `vault_backlinks` | Notes that link to a note |
| `vault_write` | Create a note, or replace one (pass its version) |
| `vault_append` | Add text to the end of a note |
| `vault_edit` | Replace one exact passage (pass its version) |

Changes are confined to the vault, rejected if the note changed since it was read (`stale_version`), written atomically under a lock, and logged to `<vault>/.atrium-memory/audit.jsonl` without note content. The format is described in [SPEC.md](SPEC.md).

## Use it from an MCP client

Add this to the client's MCP configuration (for Claude Code, a project's `.mcp.json`):

```json
{
  "mcpServers": {
    "atrium-memory": {
      "command": "/path/to/atrium-memory",
      "args": ["mcp"],
      "env": {
        "ATRIUM_MEMORY_PATH": "/path/to/your/vault",
        "ATRIUM_MEMORY_ACTOR": "my-agent"
      }
    }
  }
}
```

`ATRIUM_MEMORY_ACTOR` names the caller in the audit log.

## Command line

```sh
atrium-memory search --vault ~/Vault "langgraph agents"
atrium-memory read --vault ~/Vault "Notes/LangGraph basics.md"
atrium-memory doctor --vault ~/Vault
```

Run `atrium-memory help` for every command.

## Limits

- The write lock and atomic appends assume a local file system. They are not safe on NFS, or on a vault that iCloud or Dropbox sync shares between machines.
- The audit log grows without rotation.
- Calls rejected by input validation, before they reach a tool, are not logged.
- Note names containing `:` can't be addressed.

## Build

```sh
go test ./...
go build ./cmd/atrium-memory
```

## Licence

MIT. See [LICENSE](LICENSE).
