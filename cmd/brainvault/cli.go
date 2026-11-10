package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/poom-h/brainvault/internal/audit"
	"github.com/poom-h/brainvault/internal/vault"
)

// cli runs one command against a vault and prints JSON. Calls are logged like MCP calls.
type cli struct {
	vault *vault.Vault
	log   *audit.Log
	actor string
	out   io.Writer
}

func (c cli) record(tool string, e audit.Entry, err error) {
	_ = c.log.Call(c.actor, tool, e, vault.OutcomeOf(err)) // a CLI call still succeeds if the log can't be written
}

func (c cli) print(v any) error {
	enc := json.NewEncoder(c.out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func oneID(args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("expected exactly one note ID (put flags before it)")
	}
	return args[0], nil
}

func (c cli) search(query, folder string, limit int) error {
	matches, err := c.vault.Search(query, folder, limit)
	c.record(audit.ToolSearch, audit.Entry{Query: query, Folder: folder, Results: vault.MatchIDs(matches)}, err)
	if err != nil {
		return err
	}
	return c.print(matches)
}

func (c cli) read(args []string) error {
	id, err := oneID(args)
	if err != nil {
		return err
	}
	note, err := c.vault.Read(id)
	c.record(audit.ToolRead, audit.Entry{ID: note.IDOr(id)}, err)
	if err != nil {
		return err
	}
	return c.print(note)
}

func (c cli) list(folder string, recursive bool) error {
	notes, err := c.vault.List(folder, recursive)
	c.record(audit.ToolList, audit.Entry{Folder: folder}, err)
	if err != nil {
		return err
	}
	return c.print(notes)
}

func (c cli) backlinks(args []string) error {
	id, err := oneID(args)
	if err != nil {
		return err
	}
	ids, err := c.vault.Backlinks(id)
	c.record(audit.ToolBacklinks, audit.Entry{ID: id}, err)
	if err != nil {
		return err
	}
	return c.print(ids)
}

func (c cli) write(args []string, version string, in io.Reader) error {
	id, err := oneID(args)
	if err != nil {
		return err
	}
	content, err := io.ReadAll(in)
	if err != nil {
		return err
	}
	saved, err := c.vault.Write(id, string(content), version)
	c.record(audit.ToolWrite, audit.Entry{ID: saved.IDOr(id)}, err)
	if err != nil {
		return err
	}
	return c.print(saved)
}

// doctor prints the vault check and reports whether it found no errors.
func (c cli) doctor() (bool, error) {
	r, err := c.vault.Check()
	if err != nil {
		return false, err
	}
	return len(r.Errors) == 0, c.print(r)
}
