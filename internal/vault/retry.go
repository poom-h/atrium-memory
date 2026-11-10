package vault

import "time"

const (
	renameBudget = 500 * time.Millisecond // how long a replace waits for a note another program has open
	renameStep   = 25 * time.Millisecond
)

// renameWithRetry calls rename, and while it fails with an error retryable accepts, waits renameStep and tries
// again, for about renameBudget in total. It returns the last error.
func renameWithRetry(oldPath, newPath string, rename func(oldPath, newPath string) error, retryable func(error) bool, sleep func(time.Duration)) error {
	var waited time.Duration
	for {
		err := rename(oldPath, newPath)
		if err == nil || !retryable(err) || waited >= renameBudget {
			return err
		}
		sleep(renameStep)
		waited += renameStep
	}
}
