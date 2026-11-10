# atrium-memory vault format

A vault is a folder of Markdown notes. Any program that follows these rules can read and write a vault safely alongside atrium-memory and Obsidian.

## Notes

- A note is a UTF-8 file ending in `.md`, anywhere under the vault folder.
- Files and folders whose names start with `.` are not notes. They are never listed, searched or written. (`.obsidian/`, `.atrium-memory/` and `.tools/` are examples.)
- A note's **ID** is its path relative to the vault folder, with `/` between parts on every OS and Unicode NFC normalisation, for example `Notes/LangGraph basics.md`.
- IDs match existing files ignoring case and Unicode normalisation. Two notes whose IDs differ only in case are not allowed.
- A note's **title** is its file name without `.md`.
- A note's **version** is the lowercase hex SHA-256 of its file bytes.

## Properties

A note may start with a YAML properties block between two lines containing only `---`:

```
---
tags: [ai, agents]
created: 2026-11-07
---
```

## Links

- Links use Obsidian syntax: `[[Target]]`, `[[Target|label]]`, `[[Target#Heading]]`.
- `Target` may end in `.md`, and may contain a folder path (`[[Notes/Alpha]]`).
- A target with a folder path first matches the note with that exact ID (ignoring case). Otherwise, and as a fallback, it matches notes whose title equals the target's last part (ignoring case).
- If several notes share that title, the one with the shortest ID wins (then the alphabetically first).

## Line endings

Existing notes keep their line-ending style (CRLF or LF) when changed. New notes use LF.

## Tool files

atrium-memory keeps its own files in `<vault>/.atrium-memory/`: `audit.jsonl` (one JSON object per tool call, never including note content) and `write.lock`.

## Suggested layout

ClaudeDesk creates `Inbox/`, `Notes/`, `Maps/`, `Assets/`, `Archive/` and `Home.md`. atrium-memory accepts any layout.
