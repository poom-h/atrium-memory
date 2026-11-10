//go:build !windows

package vault

import "os"

// replaceFile renames a finished temp file over a note.
func replaceFile(oldPath, newPath string) error { return os.Rename(oldPath, newPath) }

// isBusy reports whether a replace failed because another program has the note open. Only Windows has that.
func isBusy(error) bool { return false }
