package vault

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// Vault is an open Memory Vault folder.
type Vault struct {
	Root string // absolute path with symlinks resolved
}

// Open checks that dir is a folder and returns the vault rooted there.
func Open(dir string) (*Vault, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, fmt.Errorf("vault folder %s: %w", dir, err)
	}
	info, err := os.Stat(real)
	if err != nil {
		return nil, fmt.Errorf("vault folder %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("vault folder %s is not a folder", dir)
	}
	if _, err := os.ReadDir(real); err != nil {
		return nil, fmt.Errorf("vault folder %s cannot be read: %w", dir, cause(err))
	}
	return &Vault{Root: real}, nil
}

// target is a note ID checked against the vault.
type target struct {
	ID     string // vault-relative, forward slashes, NFC, spelled as on disk where it exists
	Abs    string // absolute path on disk
	Exists bool
}

// resolve checks a note ID and finds its file. Existing files and folders match ignoring case and
// Unicode normalisation. The file does not need to exist.
func (v *Vault) resolve(id string) (target, error) {
	raw := strings.TrimSpace(strings.ReplaceAll(id, "\\", "/"))
	if raw == "" {
		return target{}, errorf(OutsideVault, "note ID is empty; use a vault-relative path such as Notes/Example.md")
	}
	if strings.HasPrefix(raw, "/") || strings.Contains(raw, ":") {
		return target{}, errorf(OutsideVault, "%q is an absolute path; use a vault-relative note ID such as Notes/Example.md", id)
	}
	clean := path.Clean(norm.NFC.String(raw))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return target{}, errorf(OutsideVault, "%q points outside the vault", id)
	}
	parts := strings.Split(clean, "/")
	for _, p := range parts {
		if strings.HasPrefix(p, ".") {
			return target{}, errorf(OutsideVault, "%q is hidden; folders and files starting with . are not notes", id)
		}
	}
	if strings.ToLower(path.Ext(clean)) != ".md" {
		return target{}, errorf(InvalidNote, "%q is not a Markdown note; note IDs end in .md", id)
	}

	abs, exists := v.Root, true
	idParts := make([]string, len(parts))
	for i, p := range parts {
		name := p
		if exists {
			found, ok, err := lookup(abs, p)
			if err != nil {
				return target{}, errorf(Internal, "cannot list the folder %q to look for %q: %s", folderID(idParts[:i]), id, cause(err))
			}
			if ok {
				name = found
			} else {
				exists = false
			}
		}
		abs = filepath.Join(abs, name)
		idParts[i] = norm.NFC.String(name)
	}
	if err := v.checkInside(abs); err != nil {
		return target{}, err
	}
	if exists {
		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			return target{}, errorf(InvalidNote, "%q is a folder, not a note", id)
		}
	}
	return target{ID: strings.Join(idParts, "/"), Abs: abs, Exists: exists}, nil
}

// lookup finds the entry of dir called name: exactly after NFC normalisation, or else ignoring case.
// A folder that does not exist means the entry is not there; any other failure to list it is an error,
// because guessing "not there" could make a create overwrite an existing note.
func lookup(dir, name string) (string, bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	for _, e := range entries {
		if norm.NFC.String(e.Name()) == name {
			return e.Name(), true, nil
		}
	}
	for _, e := range entries {
		if strings.EqualFold(norm.NFC.String(e.Name()), name) {
			return e.Name(), true, nil
		}
	}
	return "", false, nil
}

// folderID is a vault-relative folder for messages; "" is the vault root.
func folderID(parts []string) string {
	if len(parts) == 0 {
		return "."
	}
	return strings.Join(parts, "/")
}

// cause is the reason inside an OS error, without the absolute path it carries.
func cause(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	var le *os.LinkError
	if errors.As(err, &le) {
		return le.Err
	}
	return err
}

// checkInside rejects a path whose deepest existing part resolves, through symlinks, outside the vault.
func (v *Vault) checkInside(abs string) error {
	p := abs
	for {
		if _, err := os.Lstat(p); err == nil {
			break
		}
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		p = parent
	}
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		return errorf(OutsideVault, "cannot resolve the note's location")
	}
	rel, err := filepath.Rel(v.Root, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errorf(OutsideVault, "the note's location leads outside the vault through a link")
	}
	return nil
}
