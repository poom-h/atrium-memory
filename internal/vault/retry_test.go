package vault

import (
	"errors"
	"testing"
	"time"
)

var errBusy = errors.New("busy")

func TestRenameWithRetrySucceedsAfterBusy(t *testing.T) {
	calls, slept := 0, time.Duration(0)
	rename := func(_, _ string) error {
		calls++
		if calls < 4 {
			return errBusy
		}
		return nil
	}
	err := renameWithRetry("a", "b", rename, func(e error) bool { return e == errBusy }, func(d time.Duration) { slept += d })
	if err != nil || calls != 4 {
		t.Fatalf("err = %v after %d calls, want success on the 4th", err, calls)
	}
	if slept == 0 {
		t.Fatal("never waited between attempts")
	}
}

func TestRenameWithRetryGivesUpAfterBudget(t *testing.T) {
	calls, slept := 0, time.Duration(0)
	rename := func(_, _ string) error { calls++; return errBusy }
	err := renameWithRetry("a", "b", rename, func(error) bool { return true }, func(d time.Duration) { slept += d })
	if err != errBusy {
		t.Fatalf("err = %v, want the last rename error", err)
	}
	if slept < renameBudget || slept > renameBudget+renameStep {
		t.Fatalf("waited %v in total, want about %v", slept, renameBudget)
	}
	if calls < 2 {
		t.Fatalf("only %d attempts", calls)
	}
}

func TestRenameWithRetryDoesNotRetryOtherErrors(t *testing.T) {
	calls := 0
	other := errors.New("disk on fire")
	rename := func(_, _ string) error { calls++; return other }
	err := renameWithRetry("a", "b", rename, func(e error) bool { return e == errBusy }, func(time.Duration) { t.Fatal("slept") })
	if err != other || calls != 1 {
		t.Fatalf("err = %v after %d calls, want one attempt", err, calls)
	}
}
