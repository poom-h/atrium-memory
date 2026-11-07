package vault

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/poom-h/brainvault/internal/lock"
)

const lockWait = 5 * time.Second

// Saved is the result of a change: the note's ID as stored and its new version.
type Saved struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

// Write creates a note (version "") or replaces a whole note (version from the last read).
func (v *Vault) Write(id, content, version string) (Saved, error) {
	var saved Saved
	err := v.withLock(func() error {
		t, err := v.resolve(id)
		if err != nil {
			return err
		}
		if !utf8.ValidString(content) {
			return errorf(InvalidNote, "content is not valid UTF-8 text")
		}
		if !t.Exists {
			if version != "" {
				return errorf(NotFound, "no note %q to replace; omit version to create it", id)
			}
			saved, err = writeAtomic(t, []byte(toLF(content)), 0o644)
			return err
		}
		old, err := readNote(t)
		if err != nil {
			return err
		}
		current := versionOf(old)
		if version == "" {
			return &Error{Code: Exists, Message: fmt.Sprintf("%q already exists; read it and pass its version to replace it", t.ID), CurrentVersion: current}
		}
		if version != current {
			return stale(t.ID, current)
		}
		saved, err = writeAtomic(t, []byte(restoreEOL(toLF(content), old)), modeOf(t.Abs))
		return err
	})
	return saved, err
}

// Append adds text on a new line at the end of a note, creating it if missing. version is optional.
func (v *Vault) Append(id, text, version string) (Saved, error) {
	var saved Saved
	err := v.withLock(func() error {
		t, err := v.resolve(id)
		if err != nil {
			return err
		}
		if !utf8.ValidString(text) {
			return errorf(InvalidNote, "text is not valid UTF-8")
		}
		if !t.Exists {
			if version != "" {
				return errorf(NotFound, "no note %q; omit version to create it", id)
			}
			saved, err = writeAtomic(t, []byte(ensureNewline(toLF(text))), 0o644)
			return err
		}
		old, err := readNote(t)
		if err != nil {
			return err
		}
		if current := versionOf(old); version != "" && version != current {
			return stale(t.ID, current)
		}
		s := toLF(string(old))
		if s != "" && !strings.HasSuffix(s, "\n") {
			s += "\n"
		}
		s += ensureNewline(toLF(text))
		saved, err = writeAtomic(t, []byte(restoreEOL(s, old)), modeOf(t.Abs))
		return err
	})
	return saved, err
}

// Edit replaces the one place oldText appears in a note with newText. version is required.
func (v *Vault) Edit(id, oldText, newText, version string) (Saved, error) {
	var saved Saved
	err := v.withLock(func() error {
		t, err := v.resolve(id)
		if err != nil {
			return err
		}
		if !t.Exists {
			return errorf(NotFound, "no note %q", id)
		}
		if !utf8.ValidString(newText) {
			return errorf(InvalidNote, "new_text is not valid UTF-8")
		}
		old, err := readNote(t)
		if err != nil {
			return err
		}
		current := versionOf(old)
		if version == "" {
			return &Error{Code: StaleVersion, Message: "pass the version from your last vault_read of this note", CurrentVersion: current}
		}
		if version != current {
			return stale(t.ID, current)
		}
		s, find := toLF(string(old)), toLF(oldText)
		if find == "" {
			return errorf(NoMatch, "old_text is empty")
		}
		switch n := strings.Count(s, find); {
		case n == 0:
			return errorf(NoMatch, "old_text does not appear in %q; read the note again and copy the passage exactly", t.ID)
		case n > 1:
			return errorf(AmbiguousMatch, "old_text appears %d times in %q; include more surrounding text so it appears once", n, t.ID)
		}
		s = strings.Replace(s, find, toLF(newText), 1)
		saved, err = writeAtomic(t, []byte(restoreEOL(s, old)), modeOf(t.Abs))
		return err
	})
	return saved, err
}

func stale(id, current string) *Error {
	return &Error{Code: StaleVersion, Message: fmt.Sprintf("%q changed since you read it; read it again, merge your change, and retry", id), CurrentVersion: current}
}

// withLock runs fn while holding the vault's write lock.
func (v *Vault) withLock(fn func() error) error {
	dir := filepath.Join(v.Root, ".brainvault")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	l, err := lock.Acquire(filepath.Join(dir, "write.lock"), lockWait)
	if errors.Is(err, lock.ErrTimeout) {
		return errorf(Locked, "another writer is holding the vault; retry shortly")
	}
	if err != nil {
		return err
	}
	defer l.Release()
	return fn()
}

// writeAtomic writes data to a temp file beside the note, then renames it over the note.
func writeAtomic(t target, data []byte, mode fs.FileMode) (Saved, error) {
	dir := filepath.Dir(t.Abs)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Saved{}, err
	}
	tmp, err := os.CreateTemp(dir, ".brainvault-*.tmp")
	if err != nil {
		return Saved{}, err
	}
	name := tmp.Name()
	defer os.Remove(name) // does nothing after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return Saved{}, err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return Saved{}, err
	}
	if err := tmp.Close(); err != nil {
		return Saved{}, err
	}
	if err := os.Chmod(name, mode); err != nil {
		return Saved{}, err
	}
	if err := os.Rename(name, t.Abs); err != nil {
		return Saved{}, err
	}
	return Saved{ID: t.ID, Version: versionOf(data)}, nil
}

func modeOf(abs string) fs.FileMode {
	if info, err := os.Stat(abs); err == nil {
		return info.Mode().Perm()
	}
	return 0o644
}

func toLF(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }

// restoreEOL gives LF text the line endings of the original file.
func restoreEOL(lf string, original []byte) string {
	if bytes.Contains(original, []byte("\r\n")) {
		return strings.ReplaceAll(lf, "\n", "\r\n")
	}
	return lf
}

func ensureNewline(s string) string {
	if strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}
