package vault

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/poom-h/brainvault/internal/lock"
)

const mixed = "one\r\ntwo\nthree\r\nfour\nfive\n"

func TestEditRejectsOverlappingMatches(t *testing.T) {
	v := newVault(t, map[string]string{"A.md": "aaa"})
	_, err := v.Edit("A.md", "aa", "b", versionOf([]byte("aaa")))
	wantCode(t, err, AmbiguousMatch)
	if got := readFile(t, v, "A.md"); got != "aaa" {
		t.Fatalf("file changed: %q", got)
	}
}

func TestEditMixedEndingsTouchesOnlyThatLine(t *testing.T) {
	v := newVault(t, map[string]string{"M.md": mixed})
	if _, err := v.Edit("M.md", "four", "FOUR", versionOf([]byte(mixed))); err != nil {
		t.Fatal(err)
	}
	if got, want := readFile(t, v, "M.md"), "one\r\ntwo\nthree\r\nFOUR\nfive\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestEditLFOldTextMatchesCRLFNote(t *testing.T) {
	start := "a\r\nb\r\nc\r\n"
	v := newVault(t, map[string]string{"C.md": start})
	if _, err := v.Edit("C.md", "a\nb", "x\ny\nz", versionOf([]byte(start))); err != nil {
		t.Fatal(err)
	}
	if got, want := readFile(t, v, "C.md"), "x\r\ny\r\nz\r\nc\r\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestEditBothFormsPresentIsAmbiguous(t *testing.T) {
	start := "a\nb\r\na\r\nb\n"
	v := newVault(t, map[string]string{"B.md": start})
	_, err := v.Edit("B.md", "a\nb", "x", versionOf([]byte(start)))
	wantCode(t, err, AmbiguousMatch)
	if got := readFile(t, v, "B.md"); got != start {
		t.Fatalf("file changed: %q", got)
	}
}

func TestAppendMixedEndingsKeepsExistingBytes(t *testing.T) {
	v := newVault(t, map[string]string{"M.md": mixed})
	if _, err := v.Append("M.md", "six", ""); err != nil {
		t.Fatal(err)
	}
	if got, want := readFile(t, v, "M.md"), mixed+"six\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestAppendMostlyCRLFAddsCRLF(t *testing.T) {
	start := "a\r\nb\r\nc\nd"
	v := newVault(t, map[string]string{"W.md": start})
	if _, err := v.Append("W.md", "e\nf", ""); err != nil {
		t.Fatal(err)
	}
	if got, want := readFile(t, v, "W.md"), start+"\r\ne\r\nf\r\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWriteReplaceUsesDominantStyle(t *testing.T) {
	start := "a\r\nb\r\nc\n"
	v := newVault(t, map[string]string{"W.md": start})
	if _, err := v.Write("W.md", "x\ny\n", versionOf([]byte(start))); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, v, "W.md"); got != "x\r\ny\r\n" {
		t.Fatalf("got %q", got)
	}
}

func TestWriteReportsLockedWhenAnotherWriterHoldsTheVault(t *testing.T) {
	v := newVault(t, nil)
	dir := filepath.Join(v.Root, ".brainvault")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	l, err := lock.Acquire(filepath.Join(dir, "write.lock"), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Release()
	old := lockWait
	lockWait = 100 * time.Millisecond
	t.Cleanup(func() { lockWait = old })

	_, err = v.Write("A.md", "x", "")
	wantCode(t, err, Locked)
	if _, statErr := os.Stat(filepath.Join(v.Root, "A.md")); !os.IsNotExist(statErr) {
		t.Fatalf("note should not exist, stat err = %v", statErr)
	}
}

func TestWriteKeepsFileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits")
	}
	v := newVault(t, map[string]string{"P.md": "old\n"})
	p := filepath.Join(v.Root, "P.md")
	if err := os.Chmod(p, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Write("P.md", "new\n", versionOf([]byte("old\n"))); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestStaleVersionLeavesFileUnchanged(t *testing.T) {
	v := newVault(t, map[string]string{"S.md": mixed})
	_, err := v.Append("S.md", "x", "stale")
	wantCode(t, err, StaleVersion)
	_, err = v.Edit("S.md", "one", "1", "stale")
	wantCode(t, err, StaleVersion)
	if got := readFile(t, v, "S.md"); got != mixed {
		t.Fatalf("file changed: %q", got)
	}
}
