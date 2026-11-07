package vault

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenRejectsMissingFolder(t *testing.T) {
	if _, err := Open(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("Open of a missing folder succeeded")
	}
}

func TestResolve(t *testing.T) {
	v := newVault(t, map[string]string{"Notes/Alpha.md": "a", "Home.md": "h"})
	tests := []struct {
		name, id, wantID string
		wantExists       bool
		wantCode         Code
	}{
		{"exact", "Notes/Alpha.md", "Notes/Alpha.md", true, ""},
		{"case-insensitive", "notes/ALPHA.md", "Notes/Alpha.md", true, ""},
		{"backslashes", `Notes\Alpha.md`, "Notes/Alpha.md", true, ""},
		{"dot segment", "Notes/./Alpha.md", "Notes/Alpha.md", true, ""},
		{"new note", "Notes/New idea.md", "Notes/New idea.md", false, ""},
		{"new folder", "Projects/Plan.md", "Projects/Plan.md", false, ""},
		{"empty", "", "", false, OutsideVault},
		{"absolute", "/etc/passwd.md", "", false, OutsideVault},
		{"windows drive", `C:\x.md`, "", false, OutsideVault},
		{"parent", "../x.md", "", false, OutsideVault},
		{"parent inside", "Notes/../../x.md", "", false, OutsideVault},
		{"hidden folder", ".obsidian/app.md", "", false, OutsideVault},
		{"hidden file", "Notes/.secret.md", "", false, OutsideVault},
		{"not markdown", "Notes/Alpha.txt", "", false, InvalidNote},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := v.resolve(tc.id)
			if tc.wantCode != "" {
				wantCode(t, err, tc.wantCode)
				return
			}
			if err != nil {
				t.Fatalf("resolve(%q): %v", tc.id, err)
			}
			if got.ID != tc.wantID || got.Exists != tc.wantExists {
				t.Fatalf("resolve(%q) = %q exists=%v, want %q exists=%v", tc.id, got.ID, got.Exists, tc.wantID, tc.wantExists)
			}
		})
	}
}

func TestResolveRejectsFolder(t *testing.T) {
	v := newVault(t, map[string]string{"Odd.md/inner.md": "x"})
	_, err := v.resolve("Odd.md")
	wantCode(t, err, InvalidNote)
}

func TestResolveRejectsSymlinkEscape(t *testing.T) {
	v := newVault(t, nil)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.md"), []byte("s"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(v.Root, "Escape")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := v.resolve("Escape/secret.md")
	wantCode(t, err, OutsideVault)
	_, err = v.resolve("Escape/new.md")
	wantCode(t, err, OutsideVault)
}

func TestResolveMatchesUnicodeNormalisation(t *testing.T) {
	v := newVault(t, map[string]string{"Notes/Cafe\u0301.md": "c"}) // decomposed é on disk
	got, err := v.resolve("Notes/Caf\u00e9.md")                     // composed é from the caller
	if err != nil {
		t.Fatal(err)
	}
	if !got.Exists || got.ID != "Notes/Caf\u00e9.md" {
		t.Fatalf("got %+v, want existing note with NFC ID", got)
	}
}
