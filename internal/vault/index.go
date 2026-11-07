package vault

import (
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"
)

// Entry is one note in a listing.
type Entry struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Modified string `json:"modified"` // RFC 3339, UTC
	abs      string
}

// notes walks the vault and returns every note, sorted by ID. Hidden entries and symlinks are skipped.
func (v *Vault) notes() ([]Entry, error) {
	out := []Entry{}
	err := filepath.WalkDir(v.Root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip parts we can't read
		}
		if p == v.Root {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || d.Type()&fs.ModeSymlink != 0 || strings.ToLower(filepath.Ext(d.Name())) != ".md" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(v.Root, p)
		if err != nil {
			return nil
		}
		id := norm.NFC.String(filepath.ToSlash(rel))
		out = append(out, Entry{ID: id, Title: titleOf(id), Modified: info.ModTime().UTC().Format(time.RFC3339), abs: p})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, err
}

// List returns the notes under folder ("" for the whole vault). With recursive false, only notes directly in it.
func (v *Vault) List(folder string, recursive bool) ([]Entry, error) {
	prefix, err := cleanFolder(folder)
	if err != nil {
		return nil, err
	}
	all, err := v.notes()
	if err != nil {
		return nil, err
	}
	out := []Entry{}
	for _, e := range all {
		rest := e.ID
		if prefix != "" {
			if len(e.ID) <= len(prefix) || e.ID[len(prefix)] != '/' || !strings.EqualFold(e.ID[:len(prefix)], prefix) {
				continue
			}
			rest = e.ID[len(prefix)+1:]
		}
		if !recursive && strings.Contains(rest, "/") {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

// cleanFolder turns a folder argument into a vault-relative prefix without slashes at either end.
func cleanFolder(folder string) (string, error) {
	f := strings.Trim(strings.ReplaceAll(strings.TrimSpace(folder), "\\", "/"), "/")
	if f == "" || f == "." {
		return "", nil
	}
	c := path.Clean(norm.NFC.String(f))
	if c == ".." || strings.HasPrefix(c, "../") || strings.Contains(c, ":") {
		return "", errorf(OutsideVault, "folder %q is outside the vault", folder)
	}
	return c, nil
}
