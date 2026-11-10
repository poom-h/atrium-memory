# brainvault

The Memory Vault as a tool. `brainvault` serves a folder of Markdown notes (an Obsidian-style vault) to AI agents as MCP tools, and offers the same operations on the command line. It runs on macOS, Windows and Linux as a single binary.

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

Changes are confined to the vault, rejected if the note changed since it was read (`stale_version`), written atomically under a lock, and logged to `<vault>/.brainvault/audit.jsonl` without note content. The format is described in [SPEC.md](SPEC.md).

## Use it from an MCP client

Add this to the client's MCP configuration (for Claude Code, a project's `.mcp.json`):

```json
{
  "mcpServers": {
    "brainvault": {
      "command": "/path/to/brainvault",
      "args": ["mcp"],
      "env": {
        "BRAINVAULT_PATH": "/path/to/your/vault",
        "BRAINVAULT_ACTOR": "my-agent"
      }
    }
  }
}
```

`BRAINVAULT_ACTOR` names the caller in the audit log.

## Command line

```sh
brainvault search --vault ~/Vault "langgraph agents"
brainvault read --vault ~/Vault "Notes/LangGraph basics.md"
brainvault doctor --vault ~/Vault
```

Run `brainvault help` for every command.

## Build

```sh
go test ./...
go build ./cmd/brainvault
```
