package vault

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	v := newVault(t, map[string]string{
		"Bad.md":         "---\nkey: [unclosed\n---\nbody\n",
		"Ok.md":          "[[Nowhere]] [[Alpha]]\n",
		"Alpha.md":       "",
		"Notes/Alpha.md": "",
		"Binary.md":      "\xff",
	})
	r, err := v.Check()
	if err != nil {
		t.Fatal(err)
	}
	if r.Notes != 5 {
		t.Fatalf("notes = %d, want 5", r.Notes)
	}
	joinedErrors := strings.Join(r.Errors, "\n")
	for _, want := range []string{"Bad.md: properties block", "Binary.md: not valid UTF-8"} {
		if !strings.Contains(joinedErrors, want) {
			t.Errorf("errors missing %q:\n%s", want, joinedErrors)
		}
	}
	joinedWarnings := strings.Join(r.Warnings, "\n")
	for _, want := range []string{"Ok.md: link [[Nowhere]] has no matching note", `notes share the name "alpha"`} {
		if !strings.Contains(joinedWarnings, want) {
			t.Errorf("warnings missing %q:\n%s", want, joinedWarnings)
		}
	}
}

// lockFolder makes a vault folder unreadable until the test ends.
func lockFolder(t *testing.T, dir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("chmod 000 does not make a folder unreadable on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root can read any folder")
	}
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
}

func TestCheckReportsUnreadableFolder(t *testing.T) {
	v := newVault(t, map[string]string{"Ok.md": "fine\n", "Locked/Secret.md": "hidden\n", "Open/Deep/Note.md": "x\n"})
	lockFolder(t, filepath.Join(v.Root, "Locked"))

	r, err := v.Check()
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Errors) != 1 || !strings.HasPrefix(r.Errors[0], "Locked: cannot read: ") {
		t.Fatalf("errors = %q, want one 'Locked: cannot read: ...'", r.Errors)
	}
	if r.Notes != 2 {
		t.Fatalf("notes = %d, want 2", r.Notes)
	}
	// Everything else keeps skipping what it can't read.
	list, err := v.List("", true)
	if err != nil || len(list) != 2 {
		t.Fatalf("List = %v, %v; want the 2 readable notes", list, err)
	}
}

func TestVaultRootRemovedAfterOpenIsAnError(t *testing.T) {
	v := newVault(t, map[string]string{"Ok.md": "fine\n"})
	if err := os.RemoveAll(v.Root); err != nil {
		t.Fatal(err)
	}
	if list, err := v.List("", true); err == nil {
		t.Fatalf("List on a removed vault = %v, nil; want an error", list)
	}
	if _, err := v.Search("fine", "", 0); err == nil {
		t.Fatal("Search on a removed vault: want an error")
	}
	if _, err := v.Check(); err == nil {
		t.Fatal("Check on a removed vault: want an error")
	}
}

func TestUnreadableVaultRootIsAnError(t *testing.T) {
	v := newVault(t, map[string]string{"Ok.md": "fine\n"})
	lockFolder(t, v.Root)
	if list, err := v.List("", true); err == nil {
		t.Fatalf("List on an unreadable root = %v, nil; want an error", list)
	}
}
