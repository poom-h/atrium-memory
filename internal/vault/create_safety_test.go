package vault

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func skipUnlessPermsApply(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("folder modes don't restrict listing on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores folder modes")
	}
}

// A folder that can be entered but not listed must not make an existing note look new.
func TestCreateNeverOverwritesInUnlistableFolder(t *testing.T) {
	skipUnlessPermsApply(t)
	v := newVault(t, map[string]string{"Notes/x.md": "precious"})
	dir := filepath.Join(v.Root, "Notes")
	if err := os.Chmod(dir, 0o311); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	_, err := v.Write("Notes/x.md", "clobbered", "")
	if err == nil {
		t.Fatal("create over an existing note in an unlistable folder succeeded")
	}
	if got := CodeOf(err); got != Internal {
		t.Fatalf("code = %s (%v), want internal", got, err)
	}
	if strings.Contains(err.Error(), v.Root) {
		t.Fatalf("error leaks the vault path: %v", err)
	}
	if _, err := v.Append("Notes/x.md", "more", ""); err == nil {
		t.Fatal("append in an unlistable folder succeeded")
	}
	os.Chmod(dir, 0o755)
	if b, _ := os.ReadFile(filepath.Join(dir, "x.md")); string(b) != "precious" {
		t.Fatalf("note was changed: %q", b)
	}
}

// A note an outside program creates after resolve but before publish must not be replaced.
func TestCreateLosesRaceToExternalCreator(t *testing.T) {
	v := newVault(t, map[string]string{"Home.md": "# Home\n"})
	abs := filepath.Join(v.Root, "Notes", "Race.md")
	beforePublish = func() {
		os.MkdirAll(filepath.Dir(abs), 0o755)
		os.WriteFile(abs, []byte("external"), 0o644)
	}
	t.Cleanup(func() { beforePublish = nil })

	_, err := v.Write("Notes/Race.md", "mine", "")
	wantCode(t, err, Exists)
	if e := err.(*Error); e.CurrentVersion != versionOf([]byte("external")) {
		t.Fatalf("current version = %q, want the external note's", e.CurrentVersion)
	}
	if b, _ := os.ReadFile(abs); string(b) != "external" {
		t.Fatalf("external note was replaced: %q", b)
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(abs), ".brainvault-*.tmp")); len(left) != 0 {
		t.Fatalf("temp files left behind: %v", left)
	}
}

func TestFileSymlinkOutsideVaultRejected(t *testing.T) {
	v := newVault(t, map[string]string{"Home.md": "# Home\n"})
	outside := filepath.Join(t.TempDir(), "secret.md")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(v.Root, "Link.md")); err != nil {
		t.Skipf("cannot create symlinks here: %v", err)
	}
	_, err := v.Read("Link.md")
	wantCode(t, err, OutsideVault)
	_, err = v.Write("Link.md", "x", "")
	wantCode(t, err, OutsideVault)
	if b, _ := os.ReadFile(outside); string(b) != "secret" {
		t.Fatalf("outside file changed: %q", b)
	}
}

func TestDanglingSymlinkRejected(t *testing.T) {
	v := newVault(t, map[string]string{"Home.md": "# Home\n"})
	missing := filepath.Join(t.TempDir(), "nothing.md")
	if err := os.Symlink(missing, filepath.Join(v.Root, "Dangling.md")); err != nil {
		t.Skipf("cannot create symlinks here: %v", err)
	}
	_, err := v.Read("Dangling.md")
	wantCode(t, err, OutsideVault)
	_, err = v.Write("Dangling.md", "x", "")
	wantCode(t, err, OutsideVault)
	if _, err := os.Stat(missing); err == nil {
		t.Fatal("write followed the dangling link and created the target")
	}
}

// Where hard links fail, publishNew falls back to an exclusive create. A missing temp file makes Link fail.
func TestPublishNewFallsBackToExclusiveCreate(t *testing.T) {
	dir := t.TempDir()
	tg := target{ID: "x.md", Abs: filepath.Join(dir, "x.md")}
	if err := publishNew(filepath.Join(dir, "no-such-temp"), tg, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(tg.Abs); string(b) != "hello" {
		t.Fatalf("content = %q", b)
	}
	err := publishNew(filepath.Join(dir, "no-such-temp"), tg, []byte("again"), 0o644)
	wantCode(t, err, Exists)
	if b, _ := os.ReadFile(tg.Abs); string(b) != "hello" {
		t.Fatalf("fallback replaced the note: %q", b)
	}
}
