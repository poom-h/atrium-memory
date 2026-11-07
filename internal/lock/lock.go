// Package lock provides an exclusive, cross-process file lock with a bounded wait.
package lock

import (
	"errors"
	"os"
	"time"
)

// ErrTimeout means another holder kept the lock for the whole wait.
var ErrTimeout = errors.New("lock: timed out waiting for the lock")

// Lock is a held lock. Release it when done.
type Lock struct{ f *os.File }

// Acquire takes an exclusive lock on the file at path, creating the file if needed. It retries until wait elapses.
func Acquire(path string, wait time.Duration) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(wait)
	for {
		ok, err := tryLock(f)
		if err != nil {
			f.Close()
			return nil, err
		}
		if ok {
			return &Lock{f: f}, nil
		}
		if time.Now().After(deadline) {
			f.Close()
			return nil, ErrTimeout
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// Release unlocks and closes the lock file.
func (l *Lock) Release() error {
	err := unlock(l.f)
	if cerr := l.f.Close(); err == nil {
		err = cerr
	}
	return err
}
