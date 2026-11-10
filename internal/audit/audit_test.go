package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecordAppendsJSONLines(t *testing.T) {
	root := t.TempDir()
	l := New(root)
	l.now = func() time.Time { return time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC) }

	if err := l.Record(Entry{Actor: "atlas", Tool: "vault_read", ID: "Home.md", Outcome: "ok"}); err != nil {
		t.Fatal(err)
	}
	if err := l.Record(Entry{Actor: "atlas", Tool: "vault_search", Query: "graph", Results: []string{"A.md"}, Outcome: "ok"}); err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, ".brainvault", "audit.jsonl"); l.Path() != want {
		t.Fatalf("Path() = %s, want %s", l.Path(), want)
	}
	data, err := os.ReadFile(l.Path())
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2:\n%s", len(lines), data)
	}
	var first Entry
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatal(err)
	}
	if first.Time != "2026-11-07T09:00:00Z" || first.ID != "Home.md" || first.Actor != "atlas" {
		t.Fatalf("first = %+v", first)
	}
	if strings.Contains(lines[0], "query") {
		t.Fatalf("empty fields should be omitted: %s", lines[0])
	}
}

func TestCallSetsActorToolAndOutcome(t *testing.T) {
	l := New(t.TempDir())
	if err := l.Call("atlas", ToolRead, Entry{ID: "Home.md", Actor: "ignored", Tool: "ignored"}, "stale_version"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(l.Path())
	if err != nil {
		t.Fatal(err)
	}
	var e Entry
	if err := json.Unmarshal(data, &e); err != nil {
		t.Fatal(err)
	}
	if e.Actor != "atlas" || e.Tool != "vault_read" || e.Outcome != "stale_version" || e.ID != "Home.md" {
		t.Fatalf("entry = %+v", e)
	}
}

func TestToolNames(t *testing.T) {
	want := map[string]string{
		ToolSearch: "vault_search", ToolRead: "vault_read", ToolList: "vault_list", ToolBacklinks: "vault_backlinks",
		ToolWrite: "vault_write", ToolAppend: "vault_append", ToolEdit: "vault_edit",
	}
	for got, name := range want {
		if got != name {
			t.Errorf("tool constant %q, want %q", got, name)
		}
	}
}
