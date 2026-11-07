package vault

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorText(t *testing.T) {
	err := &Error{Code: StaleVersion, Message: "note changed", CurrentVersion: "abc"}
	if got := err.Error(); got != "stale_version: note changed (current_version: abc)" {
		t.Fatalf("got %q", got)
	}
	if got := errorf(NotFound, "no note %q", "A.md").Error(); got != `not_found: no note "A.md"` {
		t.Fatalf("got %q", got)
	}
}

func TestCodeOf(t *testing.T) {
	wrapped := fmt.Errorf("context: %w", errorf(Locked, "busy"))
	if got := CodeOf(wrapped); got != Locked {
		t.Fatalf("CodeOf(wrapped) = %s, want locked", got)
	}
	if got := CodeOf(errors.New("disk on fire")); got != Internal {
		t.Fatalf("CodeOf(plain) = %s, want internal", got)
	}
	if OutcomeOf(nil) != "ok" || OutcomeOf(wrapped) != "locked" {
		t.Fatalf("OutcomeOf: got %q and %q", OutcomeOf(nil), OutcomeOf(wrapped))
	}
}
