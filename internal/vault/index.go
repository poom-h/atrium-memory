package vault

import (
	"fmt"
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

// unreadable is a vault path the walk couldn't read.
type unreadable struct {
	ID  string // vault-relative, forward slashes
	Err error
}

// notes walks the vault and returns every note, sorted by ID. Hidden entries and symlinks are skipped,
// and so is anything that can't be read; only an unreadable vault root is an error.
func (v *Vault) notes() ([]Entry, error) {
	out, _, err := v.walk()
	return out, err
}

// walk is notes plus the vault paths it couldn't read, sorted by path. Check reports those.
func (v *Vault) walk() ([]Entry, []unreadable, error) {
	out := []Entry{}
	var skipped []unreadable
	var rootErr error
	skip := func(p string, err error) {
		rel, rerr := filepath.Rel(v.Root, p)
		if rerr != nil {
			rel = p
		}
		skipped = append(skipped, unreadable{ID: norm.NFC.String(filepath.ToSlash(rel)), Err: err})
	}
	_ = filepath.WalkDir(v.Root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == v.Root {
				rootErr = err
				return filepath.SkipAll
			}
			skip(p, err)
			return nil
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
			skip(p, err)
			return nil
		}
		rel, err := filepath.Rel(v.Root, p)
		if err != nil {
			skip(p, err)
			return nil
		}
		id := norm.NFC.String(filepath.ToSlash(rel))
		out = append(out, Entry{ID: id, Title: titleOf(id), Modified: info.ModTime().UTC().Format(time.RFC3339), abs: p})
		return nil
	})
	if rootErr != nil {
		return nil, nil, fmt.Errorf("cannot read the vault folder: %w", rootErr)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	sort.Slice(skipped, func(i, j int) bool { return skipped[i].ID < skipped[j].ID })
	return out, skipped, nil
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
