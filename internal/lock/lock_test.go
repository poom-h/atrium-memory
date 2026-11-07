package lock

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireExcludesSecondHolder(t *testing.T) {
	p := filepath.Join(t.TempDir(), "write.lock")
	first, err := Acquire(p, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := Acquire(p, 100*time.Millisecond); !errors.Is(err, ErrTimeout) {
		t.Fatalf("second Acquire = %v, want ErrTimeout", err)
	}
	if time.Since(start) < 100*time.Millisecond {
		t.Fatal("gave up before the wait elapsed")
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	second, err := Acquire(p, time.Second)
	if err != nil {
		t.Fatalf("Acquire after Release: %v", err)
	}
	if err := second.Release(); err != nil {
		t.Fatal(err)
	}
}
