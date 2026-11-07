package vault

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readFile(t *testing.T, v *Vault, id string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(v.Root, filepath.FromSlash(id)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestWrite(t *testing.T) {
	v := newVault(t, map[string]string{"Notes/Alpha.md": "old\n"})

	saved, err := v.Write("Projects/New.md", "line1\r\nline2\r\n", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, v, "Projects/New.md"); got != "line1\nline2\n" {
		t.Fatalf("new note = %q, want LF line endings", got)
	}
	if saved.ID != "Projects/New.md" || saved.Version != versionOf([]byte("line1\nline2\n")) {
		t.Fatalf("saved = %+v", saved)
	}

	_, err = v.Write("Notes/Alpha.md", "x", "")
	wantCode(t, err, Exists)
	_, err = v.Write("notes/ALPHA.md", "x", "") // differs only in case
	wantCode(t, err, Exists)

	_, err = v.Write("Notes/Alpha.md", "x", "wrong")
	wantCode(t, err, StaleVersion)
	var e *Error
	if !errors.As(err, &e) || e.CurrentVersion != versionOf([]byte("old\n")) {
		t.Fatalf("stale error should carry the current version, got %v", err)
	}

	saved, err = v.Write("notes/alpha.md", "new\n", versionOf([]byte("old\n")))
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID != "Notes/Alpha.md" || readFile(t, v, "Notes/Alpha.md") != "new\n" {
		t.Fatalf("replace failed: %+v %q", saved, readFile(t, v, "Notes/Alpha.md"))
	}

	_, err = v.Write("Notes/Ghost.md", "x", "abc")
	wantCode(t, err, NotFound)
	_, err = v.Write("../escape.md", "x", "")
	wantCode(t, err, OutsideVault)
	_, err = v.Write("Notes/Bad.md", "\xff", "")
	wantCode(t, err, InvalidNote)
}

func TestWriteKeepsCRLF(t *testing.T) {
	start := "a\r\nb\r\n"
	v := newVault(t, map[string]string{"Win.md": start})
	if _, err := v.Write("Win.md", "c\nd\n", versionOf([]byte(start))); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, v, "Win.md"); got != "c\r\nd\r\n" {
		t.Fatalf("got %q, want CRLF kept", got)
	}
}

func TestAppend(t *testing.T) {
	v := newVault(t, map[string]string{"Log.md": "first"})
	if _, err := v.Append("Log.md", "second", ""); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, v, "Log.md"); got != "first\nsecond\n" {
		t.Fatalf("got %q", got)
	}
	_, err := v.Append("Log.md", "third", "stale")
	wantCode(t, err, StaleVersion)
	if _, err := v.Append("Inbox/New.md", "hello", ""); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, v, "Inbox/New.md"); got != "hello\n" {
		t.Fatalf("got %q", got)
	}
}

func TestEdit(t *testing.T) {
	start := "# Plan\r\nstep one\r\nstep two\r\n"
	v := newVault(t, map[string]string{"Plan.md": start})
	ver := versionOf([]byte(start))

	_, err := v.Edit("Plan.md", "step one", "step 1", "")
	wantCode(t, err, StaleVersion)
	_, err = v.Edit("Plan.md", "missing", "x", ver)
	wantCode(t, err, NoMatch)
	_, err = v.Edit("Plan.md", "step", "x", ver)
	wantCode(t, err, AmbiguousMatch)
	_, err = v.Edit("Plan.md", "", "x", ver)
	wantCode(t, err, NoMatch)
	_, err = v.Edit("Ghost.md", "a", "b", ver)
	wantCode(t, err, NotFound)

	if _, err := v.Edit("Plan.md", "step one\nstep two", "step 1\nstep 2", ver); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, v, "Plan.md"); got != "# Plan\r\nstep 1\r\nstep 2\r\n" {
		t.Fatalf("got %q", got)
	}
}

func TestWriteLeavesNoTempFiles(t *testing.T) {
	v := newVault(t, nil)
	if _, err := v.Write("Notes/A.md", "a", ""); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(v.Root, "Notes"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}
