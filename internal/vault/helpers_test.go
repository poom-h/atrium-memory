package vault

import (
	"os"
	"path/filepath"
	"testing"
)

// newVault makes a vault in a temp folder with the given files (vault-relative path -> content).
func newVault(t *testing.T, files map[string]string) *Vault {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	v, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func wantCode(t *testing.T, err error, code Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("got no error, want %s", code)
	}
	if got := CodeOf(err); got != code {
		t.Fatalf("got error %v (code %s), want code %s", err, got, code)
	}
}
