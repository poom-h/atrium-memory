//go:build windows

package vault

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// isBusy reports whether err means another program has the file open: Go opens files without
// FILE_SHARE_DELETE, so a rename over a note a reader holds fails with one of these.
func isBusy(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}

// replaceFile renames a finished temp file over a note, retrying briefly while another program has the note open.
func replaceFile(oldPath, newPath string) error {
	return renameWithRetry(oldPath, newPath, os.Rename, isBusy, time.Sleep)
}
