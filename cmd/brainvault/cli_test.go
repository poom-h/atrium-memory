package main

import (
	"bytes"
	"strings"
	"testing"
)

func runCLI(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := run(args, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestCLI(t *testing.T) {
	dir := makeVault(t, map[string]string{"Home.md": "# Home\n[[Alpha]]\n", "Alpha.md": "about graphs\n"})

	code, out, errOut := runCLI(t, "# Gamma\nnew note\n", "write", "--vault", dir, "--actor", "cli-test", "Notes/Gamma.md")
	if code != 0 || !strings.Contains(out, `"version"`) {
		t.Fatalf("write: code %d out %q err %q", code, out, errOut)
	}
	if code, out, _ = runCLI(t, "", "read", "--vault", dir, "Notes/Gamma.md"); code != 0 || !strings.Contains(out, "new note") {
		t.Fatalf("read: code %d out %q", code, out)
	}
	if code, out, _ = runCLI(t, "", "search", "--vault", dir, "graphs"); code != 0 || !strings.Contains(out, `"Alpha.md"`) {
		t.Fatalf("search: code %d out %q", code, out)
	}
	if code, out, _ = runCLI(t, "", "backlinks", "--vault", dir, "Alpha.md"); code != 0 || !strings.Contains(out, "Home.md") {
		t.Fatalf("backlinks: code %d out %q", code, out)
	}
	if code, out, _ = runCLI(t, "", "list", "--vault", dir, "--folder", "Notes"); code != 0 || !strings.Contains(out, "Gamma") || strings.Contains(out, "Home.md") {
		t.Fatalf("list: code %d out %q", code, out)
	}
	if code, _, errOut = runCLI(t, "", "read", "--vault", dir, "../x.md"); code != 1 || !strings.Contains(errOut, "outside_vault") {
		t.Fatalf("read outside: code %d err %q", code, errOut)
	}
	if code, out, _ = runCLI(t, "", "doctor", "--vault", dir); code != 0 {
		t.Fatalf("doctor on a clean vault: code %d out %q", code, out)
	}
}

func TestDoctorExitsNonZeroOnErrors(t *testing.T) {
	dir := makeVault(t, map[string]string{"Bad.md": "---\nkey: [unclosed\n---\nbody\n"})
	code, out, _ := runCLI(t, "", "doctor", "--vault", dir)
	if code != 1 || !strings.Contains(out, "Bad.md: properties block") {
		t.Fatalf("doctor: code %d out %q", code, out)
	}
}

func TestCLIUsageErrors(t *testing.T) {
	if code, _, _ := runCLI(t, ""); code != 2 {
		t.Fatalf("no command: code %d, want 2", code)
	}
	if code, _, errOut := runCLI(t, "", "frobnicate", "--vault", t.TempDir()); code != 2 || !strings.Contains(errOut, "unknown command") {
		t.Fatalf("unknown command: code %d err %q", code, errOut)
	}
	t.Setenv("BRAINVAULT_PATH", "")
	if code, _, errOut := runCLI(t, "", "read", "Home.md"); code != 1 || !strings.Contains(errOut, "BRAINVAULT_PATH") {
		t.Fatalf("missing vault: code %d err %q", code, errOut)
	}
}
