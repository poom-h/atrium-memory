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

// lockWait is how long a write waits for the vault lock; tests shorten it.
var lockWait = 5 * time.Second

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
			saved, err = writeAtomic(t, []byte(toLF(content)), 0o644, true)
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
		saved, err = writeAtomic(t, []byte(withEOL(content, dominantEOL(old))), modeOf(t.Abs), false)
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
			saved, err = writeAtomic(t, []byte(ensureNewline(toLF(text))), 0o644, true)
			return err
		}
		old, err := readNote(t)
		if err != nil {
			return err
		}
		if current := versionOf(old); version != "" && version != current {
			return stale(t.ID, current)
		}
		eol := dominantEOL(old)
		out := old
		if len(out) > 0 && out[len(out)-1] != '\n' {
			out = append(out[:len(out):len(out)], eol...)
		}
		out = append(out, withEOL(ensureNewline(toLF(text)), eol)...)
		saved, err = writeAtomic(t, out, modeOf(t.Abs), false)
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
		if oldText == "" {
			return errorf(NoMatch, "old_text is empty")
		}
		s := string(old)
		find, replace, n := locateEdit(s, oldText, newText, dominantEOL(old))
		switch {
		case n == 0:
			return errorf(NoMatch, "old_text does not appear in %q; read the note again and copy the passage exactly", t.ID)
		case n > 1:
			return errorf(AmbiguousMatch, "old_text appears %d times in %q; include more surrounding text so it appears once", n, t.ID)
		}
		s = strings.Replace(s, find, replace, 1)
		saved, err = writeAtomic(t, []byte(s), modeOf(t.Abs), false)
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

// writeAtomic writes data to a temp file beside the note, then publishes it. A replace renames the temp file
// over the note (the caller has checked the version under the lock). A create (create true) must never replace
// anything: it links the temp file into place, which fails if the note exists.
func writeAtomic(t target, data []byte, mode fs.FileMode, create bool) (Saved, error) {
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
	if beforePublish != nil {
		beforePublish()
	}
	if create {
		if err := publishNew(name, t, data, mode); err != nil {
			return Saved{}, err
		}
	} else if err := replaceFile(name, t.Abs); err != nil {
		if isBusy(err) {
			return Saved{}, errorf(Locked, "the note is open in another program; retry shortly")
		}
		return Saved{}, err
	}
	return Saved{ID: t.ID, Version: versionOf(data)}, nil
}

// publishNew puts the finished temp file at t.Abs only if nothing is there. It hard-links the temp file, which
// is atomic and exclusive; where the file system has no hard links it falls back to an exclusive create
// (O_EXCL) that writes the bytes in place. The caller removes the temp file.
func publishNew(tmp string, t target, data []byte, mode fs.FileMode) error {
	err := os.Link(tmp, t.Abs)
	if err == nil {
		return nil
	}
	if errors.Is(err, fs.ErrExist) {
		return existsError(t)
	}
	f, err := os.OpenFile(t.Abs, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if errors.Is(err, fs.ErrExist) {
		return existsError(t)
	}
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(t.Abs)
		return err
	}
	return os.Chmod(t.Abs, mode)
}

// existsError is the exists error for a note that appeared while a create was under way.
func existsError(t target) *Error {
	e := &Error{Code: Exists, Message: fmt.Sprintf("%q already exists; read it and pass its version to replace it", t.ID)}
	if data, err := os.ReadFile(t.Abs); err == nil && utf8.Valid(data) {
		e.CurrentVersion = versionOf(data)
	}
	return e
}

func modeOf(abs string) fs.FileMode {
	if info, err := os.Stat(abs); err == nil {
		return info.Mode().Perm()
	}
	return 0o644
}

func toLF(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }

// locateEdit finds oldText in the raw note text in three forms: as given, in LF form and in CRLF form.
// A match with no line break gets newText in the note's dominant style; a raw match with line breaks keeps newText as given.
// It returns the one form that matched, newText in the same line-ending style, and the total match count.
func locateEdit(note, oldText, newText, noteEOL string) (find, replace string, n int) {
	forms := []struct{ find, replace string }{
		{oldText, newText},
		{withEOL(oldText, "\n"), withEOL(newText, "\n")},
		{withEOL(oldText, "\r\n"), withEOL(newText, "\r\n")},
	}
	for i, f := range forms {
		if i > 0 && (f.find == forms[0].find || (i == 2 && f.find == forms[1].find)) {
			continue
		}
		if m := countAll(note, f.find); m > 0 {
			if n == 0 {
				find, replace = f.find, f.replace
				if !strings.Contains(find, "\n") {
					replace = withEOL(newText, noteEOL)
				}
			}
			n += m
		}
	}
	return find, replace, n
}

// countAll counts every occurrence of sub in s, overlapping ones included.
func countAll(s, sub string) int {
	n := 0
	for i := 0; ; {
		j := strings.Index(s[i:], sub)
		if j < 0 {
			return n
		}
		n++
		i += j + 1
	}
}

// dominantEOL is "\r\n" when the text has more CRLF than bare LF line breaks, otherwise "\n".
func dominantEOL(b []byte) string {
	crlf := bytes.Count(b, []byte("\r\n"))
	if crlf > bytes.Count(b, []byte("\n"))-crlf {
		return "\r\n"
	}
	return "\n"
}

// withEOL rewrites every line break in s to eol.
func withEOL(s, eol string) string {
	s = toLF(s)
	if eol == "\n" {
		return s
	}
	return strings.ReplaceAll(s, "\n", eol)
}

func ensureNewline(s string) string {
	if strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}

// beforePublish, when set (tests only), runs just before a new note is published.
var beforePublish func()
