package vault

import (
	"path/filepath"
	"testing"
)

// Every vault in testdata/vaults must pass the doctor and be fully readable and searchable.
func TestConformanceVaults(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("..", "..", "testdata", "vaults", "*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) == 0 {
		t.Fatal("no conformance vaults in testdata/vaults")
	}
	for _, dir := range dirs {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			v, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			r, err := v.Check()
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Errors) > 0 {
				t.Fatalf("doctor errors: %v", r.Errors)
			}
			notes, err := v.List("", true)
			if err != nil || len(notes) == 0 {
				t.Fatalf("list: %d notes, %v", len(notes), err)
			}
			for _, n := range notes {
				if _, err := v.Read(n.ID); err != nil {
					t.Errorf("read %s: %v", n.ID, err)
				}
			}
			if _, err := v.Search("note", "", 20); err != nil {
				t.Errorf("search: %v", err)
			}
		})
	}
}
