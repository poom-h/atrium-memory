package vault

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
)

// TestHelperAppend is the second process in TestConcurrentAppendsLoseNothing. It does nothing on its own.
func TestHelperAppend(t *testing.T) {
	dir := os.Getenv("ATRIUM_MEMORY_HELPER_VAULT")
	if dir == "" {
		t.Skip("helper process only")
	}
	v, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		if _, err := v.Append("Log.md", fmt.Sprintf("proc-%d", i), ""); err != nil {
			t.Fatal(err)
		}
	}
}

func TestConcurrentAppendsLoseNothing(t *testing.T) {
	v := newVault(t, map[string]string{"Log.md": ""})
	helper := exec.Command(os.Args[0], "-test.run=^TestHelperAppend$")
	helper.Env = append(os.Environ(), "ATRIUM_MEMORY_HELPER_VAULT="+v.Root)
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 25; i++ {
				if _, err := v.Append("Log.md", fmt.Sprintf("g%d-%d", g, i), ""); err != nil {
					t.Error(err)
				}
			}
		}(g)
	}
	wg.Wait()
	if err := helper.Wait(); err != nil {
		t.Fatalf("helper process: %v", err)
	}
	if lines := strings.Fields(readFile(t, v, "Log.md")); len(lines) != 150 {
		t.Fatalf("got %d lines, want 150 (4×25 from goroutines + 50 from the second process)", len(lines))
	}
}
