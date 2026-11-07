// Package mcpserver exposes a vault as MCP tools. It only translates between MCP and the vault package.
package mcpserver

import (
	"context"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/poom-h/brainvault/internal/audit"
	"github.com/poom-h/brainvault/internal/vault"
)

type SearchIn struct {
	Query  string `json:"query" jsonschema:"words to find; every word must appear in the note's title, path or text"`
	Folder string `json:"folder,omitempty" jsonschema:"only search notes under this vault folder, such as Notes"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum results, default 20, at most 100"`
}

type SearchOut struct {
	Results []vault.Match `json:"results"`
}

type IDIn struct {
	ID string `json:"id" jsonschema:"note ID: the vault-relative path with forward slashes, such as Notes/Example.md"`
}

type ListIn struct {
	Folder    string `json:"folder,omitempty" jsonschema:"vault folder to list, such as Notes; omit for the whole vault"`
	Recursive *bool  `json:"recursive,omitempty" jsonschema:"include notes in subfolders; default true"`
}

type ListOut struct {
	Notes []vault.Entry `json:"notes"`
}

type BacklinksOut struct {
	IDs []string `json:"ids"`
}

type WriteIn struct {
	ID      string `json:"id" jsonschema:"note ID: the vault-relative path with forward slashes, such as Notes/Example.md"`
	Content string `json:"content" jsonschema:"the note's full Markdown"`
	Version string `json:"version,omitempty" jsonschema:"to replace an existing note, the version from your last vault_read; omit to create a new note"`
}

type AppendIn struct {
	ID      string `json:"id" jsonschema:"note ID: the vault-relative path with forward slashes, such as Notes/Example.md"`
	Text    string `json:"text" jsonschema:"Markdown to add at the end, on a new line"`
	Version string `json:"version,omitempty" jsonschema:"optional: the version from your last vault_read, to fail if the note changed"`
}

type EditIn struct {
	ID      string `json:"id" jsonschema:"note ID: the vault-relative path with forward slashes, such as Notes/Example.md"`
	OldText string `json:"old_text" jsonschema:"the exact passage to replace; it must appear exactly once"`
	NewText string `json:"new_text" jsonschema:"the replacement text"`
	Version string `json:"version" jsonschema:"the version from your last vault_read of this note"`
}

type server struct {
	vault *vault.Vault
	log   *audit.Log
	actor string
}

// New returns an MCP server with the seven vault tools.
func New(v *vault.Vault, log *audit.Log, actor, version string) *mcp.Server {
	s := &server{vault: v, log: log, actor: actor}
	srv := mcp.NewServer(&mcp.Implementation{Name: "brainvault", Version: version}, nil)
	mcp.AddTool(srv, &mcp.Tool{Name: "vault_search", Description: "Search the Memory Vault. Every word must appear in a note's title, path or text; title matches rank first. Returns note IDs, titles and snippets."}, s.search)
	mcp.AddTool(srv, &mcp.Tool{Name: "vault_read", Description: "Read one note: its Markdown, parsed properties, the notes it links to, and its version. Pass the version back when you change the note."}, s.read)
	mcp.AddTool(srv, &mcp.Tool{Name: "vault_list", Description: "List notes, optionally under one folder, with titles and last-modified times."}, s.list)
	mcp.AddTool(srv, &mcp.Tool{Name: "vault_backlinks", Description: "List the notes that link to a note with [[wikilinks]]."}, s.backlinks)
	mcp.AddTool(srv, &mcp.Tool{Name: "vault_write", Description: "Create a note, or replace a whole note. To replace, pass the version from vault_read; omit version to create. Prefer vault_edit or vault_append for small changes."}, s.write)
	mcp.AddTool(srv, &mcp.Tool{Name: "vault_append", Description: "Add text to the end of a note on a new line, creating the note if it doesn't exist."}, s.append)
	mcp.AddTool(srv, &mcp.Tool{Name: "vault_edit", Description: "Replace one exact passage of a note. old_text must appear exactly once. Pass the version from vault_read; if the note changed since, read it again and retry."}, s.edit)
	return srv
}

func (s *server) record(tool string, e audit.Entry, err error) {
	e.Actor, e.Tool, e.Outcome = s.actor, tool, vault.OutcomeOf(err)
	if lerr := s.log.Record(e); lerr != nil {
		fmt.Fprintln(os.Stderr, "brainvault: cannot write the audit log:", lerr)
	}
}

// savedID is the stored ID after a successful change, or the ID the caller asked for.
func savedID(saved vault.Saved, asked string) string {
	if saved.ID != "" {
		return saved.ID
	}
	return asked
}

func (s *server) search(_ context.Context, _ *mcp.CallToolRequest, in SearchIn) (*mcp.CallToolResult, SearchOut, error) {
	matches, err := s.vault.Search(in.Query, in.Folder, in.Limit)
	ids := make([]string, 0, len(matches))
	for _, m := range matches {
		ids = append(ids, m.ID)
	}
	s.record("vault_search", audit.Entry{Query: in.Query, Folder: in.Folder, Results: ids}, err)
	if err != nil {
		return nil, SearchOut{}, err
	}
	return nil, SearchOut{Results: matches}, nil
}

func (s *server) read(_ context.Context, _ *mcp.CallToolRequest, in IDIn) (*mcp.CallToolResult, vault.Note, error) {
	note, err := s.vault.Read(in.ID)
	id := note.ID
	if id == "" {
		id = in.ID
	}
	s.record("vault_read", audit.Entry{ID: id}, err)
	if err != nil {
		return nil, vault.Note{}, err
	}
	return nil, note, nil
}

func (s *server) list(_ context.Context, _ *mcp.CallToolRequest, in ListIn) (*mcp.CallToolResult, ListOut, error) {
	recursive := in.Recursive == nil || *in.Recursive
	notes, err := s.vault.List(in.Folder, recursive)
	s.record("vault_list", audit.Entry{Folder: in.Folder}, err)
	if err != nil {
		return nil, ListOut{}, err
	}
	return nil, ListOut{Notes: notes}, nil
}

func (s *server) backlinks(_ context.Context, _ *mcp.CallToolRequest, in IDIn) (*mcp.CallToolResult, BacklinksOut, error) {
	ids, err := s.vault.Backlinks(in.ID)
	s.record("vault_backlinks", audit.Entry{ID: in.ID}, err)
	if err != nil {
		return nil, BacklinksOut{}, err
	}
	return nil, BacklinksOut{IDs: ids}, nil
}

func (s *server) write(_ context.Context, _ *mcp.CallToolRequest, in WriteIn) (*mcp.CallToolResult, vault.Saved, error) {
	saved, err := s.vault.Write(in.ID, in.Content, in.Version)
	s.record("vault_write", audit.Entry{ID: savedID(saved, in.ID)}, err)
	if err != nil {
		return nil, vault.Saved{}, err
	}
	return nil, saved, nil
}

func (s *server) append(_ context.Context, _ *mcp.CallToolRequest, in AppendIn) (*mcp.CallToolResult, vault.Saved, error) {
	saved, err := s.vault.Append(in.ID, in.Text, in.Version)
	s.record("vault_append", audit.Entry{ID: savedID(saved, in.ID)}, err)
	if err != nil {
		return nil, vault.Saved{}, err
	}
	return nil, saved, nil
}

func (s *server) edit(_ context.Context, _ *mcp.CallToolRequest, in EditIn) (*mcp.CallToolResult, vault.Saved, error) {
	saved, err := s.vault.Edit(in.ID, in.OldText, in.NewText, in.Version)
	s.record("vault_edit", audit.Entry{ID: savedID(saved, in.ID)}, err)
	if err != nil {
		return nil, vault.Saved{}, err
	}
	return nil, saved, nil
}
