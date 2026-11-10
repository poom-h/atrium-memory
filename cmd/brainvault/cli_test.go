package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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

// readAudit parses every line of the vault's audit log as JSON.
func readAudit(t *testing.T, dir string) (lines []string, entries []map[string]any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".brainvault", "audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var e map[string]any
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("audit line is not JSON: %q: %v", line, err)
		}
		lines = append(lines, line)
		entries = append(entries, e)
	}
	return lines, entries
}

func TestCLICallsAreAudited(t *testing.T) {
	dir := makeVault(t, map[string]string{"Home.md": "# Home\n[[Alpha]]\n", "Alpha.md": "about graphs\n"})
	const nfd = "Notes/Cafe\u0301.md" // written decomposed; stored composed
	const nfc = "Notes/Caf\u00e9.md"

	steps := [][]string{
		{"write", "--vault", dir, "--actor", "cli-test", nfd},
		{"read", "--vault", dir, "--actor", "cli-test", "Alpha.md"},
		{"search", "--vault", dir, "--actor", "cli-test", "graphs"},
		{"list", "--vault", dir, "--actor", "cli-test", "--folder", "Notes"},
		{"backlinks", "--vault", dir, "--actor", "cli-test", "Alpha.md"},
		{"read", "--vault", dir, "--actor", "cli-test", "../x.md"},
	}
	for i, args := range steps {
		stdin := ""
		if args[0] == "write" {
			stdin = "secret body text\n"
		}
		code, _, errOut := runCLI(t, stdin, args...)
		wantCode := 0
		if i == len(steps)-1 {
			wantCode = 1
		}
		if code != wantCode {
			t.Fatalf("%v: code %d err %q", args, code, errOut)
		}
	}

	lines, entries := readAudit(t, dir)
	if len(entries) != len(steps) {
		t.Fatalf("got %d audit entries, want %d: %v", len(entries), len(steps), lines)
	}
	want := []struct{ tool, outcome, id string }{
		{"vault_write", "ok", nfc},
		{"vault_read", "ok", "Alpha.md"},
		{"vault_search", "ok", ""},
		{"vault_list", "ok", ""},
		{"vault_backlinks", "ok", "Alpha.md"},
		{"vault_read", "outside_vault", "../x.md"},
	}
	for i, w := range want {
		e := entries[i]
		if e["actor"] != "cli-test" || e["tool"] != w.tool || e["outcome"] != w.outcome {
			t.Errorf("entry %d = %v, want actor cli-test tool %s outcome %s", i, e, w.tool, w.outcome)
		}
		if id, _ := e["id"].(string); id != w.id {
			t.Errorf("entry %d id = %q, want %q", i, id, w.id)
		}
	}
	if res, _ := entries[2]["results"].([]any); len(res) != 1 || res[0] != "Alpha.md" {
		t.Errorf("search entry results = %v, want [Alpha.md]", entries[2]["results"])
	}
	if entries[2]["query"] != "graphs" || entries[3]["folder"] != "Notes" {
		t.Errorf("query/folder not logged: %v %v", entries[2], entries[3])
	}
	for _, line := range lines {
		if strings.Contains(line, "secret body") || strings.Contains(line, "about graphs") {
			t.Errorf("audit line contains note content: %s", line)
		}
	}
}

func TestFlagsAfterPositionalArgumentAreRejected(t *testing.T) {
	dir := makeVault(t, map[string]string{"Alpha.md": "about graphs\n"})
	cases := [][]string{
		{"search", "--vault", dir, "graphs", "--folder", "Notes"},
		{"search", "--vault", dir, "graphs", "-x"},
		{"read", "--vault", dir, "Alpha.md", "--actor", "x"},
		{"backlinks", "--vault", dir, "Alpha.md", "--actor", "x"},
		{"write", "--vault", dir, "Alpha.md", "--version", "v1"},
	}
	for _, args := range cases {
		code, out, errOut := runCLI(t, "body", args...)
		if code != 2 || out != "" || !strings.Contains(errOut, "flags must come before") {
			t.Errorf("%v: code %d out %q err %q, want exit 2 and a flags-before message", args, code, out, errOut)
		}
	}
	// Flags before the ID still work.
	if code, _, errOut := runCLI(t, "", "read", "--vault", dir, "Alpha.md"); code != 0 {
		t.Fatalf("plain read: code %d err %q", code, errOut)
	}
}
