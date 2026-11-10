package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wantInternal checks err is an internal error that names no absolute vault path.
func wantInternal(t *testing.T, v *Vault, err error, mention string) {
	t.Helper()
	wantCode(t, err, Internal)
	if _, ok := err.(*Error); !ok {
		t.Fatalf("error %T is not a vault *Error", err)
	}
	if strings.Contains(err.Error(), v.Root) || strings.Contains(err.Error(), filepath.ToSlash(v.Root)) {
		t.Fatalf("error leaks the vault path: %v", err)
	}
	if mention != "" && !strings.Contains(err.Error(), mention) {
		t.Fatalf("error %q does not name %q", err, mention)
	}
	if !strings.HasPrefix(err.Error(), "internal:") {
		t.Fatalf("error text %q does not start with its code", err)
	}
}

func TestReadFailureIsInternalWithoutAbsolutePath(t *testing.T) {
	skipUnlessPermsApply(t)
	v := newVault(t, map[string]string{"Notes/locked.md": "secret"})
	p := filepath.Join(v.Root, "Notes", "locked.md")
	if err := os.Chmod(p, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(p, 0o644) })

	_, err := v.Read("Notes/locked.md")
	wantInternal(t, v, err, "Notes/locked.md")
	_, err = v.Write("Notes/locked.md", "x", strings.Repeat("0", 64))
	wantInternal(t, v, err, "Notes/locked.md")
	_, err = v.Append("Notes/locked.md", "x", "")
	wantInternal(t, v, err, "Notes/locked.md")
	_, err = v.Edit("Notes/locked.md", "a", "b", strings.Repeat("0", 64))
	wantInternal(t, v, err, "Notes/locked.md")
}

func TestWriteFailureIsInternalWithoutAbsolutePath(t *testing.T) {
	skipUnlessPermsApply(t)
	v := newVault(t, map[string]string{"Notes/a.md": "a"})
	dir := filepath.Join(v.Root, "Notes")
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	_, err := v.Write("Notes/new.md", "x", "")
	wantInternal(t, v, err, "Notes/new.md")
	read, _ := v.Read("Notes/a.md")
	_, err = v.Append("Notes/a.md", "more", read.Version)
	wantInternal(t, v, err, "Notes/a.md")
}

func TestLockSetupFailureIsInternalWithoutAbsolutePath(t *testing.T) {
	v := newVault(t, map[string]string{"Home.md": "# Home\n"})
	// A file where the .brainvault folder should be.
	if err := os.WriteFile(filepath.Join(v.Root, ".brainvault"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := v.Write("New.md", "x", "")
	wantInternal(t, v, err, "")
}

func TestUnreadableVaultFolderIsInternalWithoutAbsolutePath(t *testing.T) {
	skipUnlessPermsApply(t)
	v := newVault(t, map[string]string{"Home.md": "# Home\n"})
	if err := os.Chmod(v.Root, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(v.Root, 0o755) })
	_, err := v.List("", true)
	wantInternal(t, v, err, "")
	_, err = v.Search("home", "", 0)
	wantInternal(t, v, err, "")
}
