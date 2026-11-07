package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var binary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "brainvault-e2e-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	binary = filepath.Join(dir, "brainvault")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "go build: %v\n%s", err, out)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// cleanEnv is this process's environment without BRAINVAULT_ settings, plus extra.
func cleanEnv(extra ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "BRAINVAULT_") {
			env = append(env, kv)
		}
	}
	return append(env, extra...)
}

func makeVault(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func connect(t *testing.T, dir string) *mcp.ClientSession {
	t.Helper()
	cmd := exec.Command(binary, "mcp")
	cmd.Env = cleanEnv("BRAINVAULT_PATH="+dir, "BRAINVAULT_ACTOR=e2e")
	client := mcp.NewClient(&mcp.Implementation{Name: "e2e", Version: "test"}, nil)
	session, err := client.Connect(context.Background(), &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

// call runs a tool and returns its structured result, or the error text when the tool failed.
func call(t *testing.T, s *mcp.ClientSession, tool string, args map[string]any) (map[string]any, string) {
	t.Helper()
	res, err := s.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	if res.IsError {
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				return nil, tc.Text
			}
		}
		return nil, "(no error text)"
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s: structured result %s: %v", tool, raw, err)
	}
	return out, ""
}

func mustCall(t *testing.T, s *mcp.ClientSession, tool string, args map[string]any) map[string]any {
	t.Helper()
	out, errText := call(t, s, tool, args)
	if errText != "" {
		t.Fatalf("%s failed: %s", tool, errText)
	}
	return out
}

func TestToolsOverStdio(t *testing.T) {
	dir := makeVault(t, map[string]string{
		"Home.md":        "# Home\nStart at [[Alpha]].\n",
		"Notes/Alpha.md": "# Alpha\nAbout graphs.\n",
	})
	s := connect(t, dir)
	ctx := context.Background()

	tools, err := s.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tool := range tools.Tools {
		names[tool.Name] = true
	}
	for _, want := range []string{"vault_search", "vault_read", "vault_list", "vault_backlinks", "vault_write", "vault_append", "vault_edit"} {
		if !names[want] {
			t.Errorf("tool %s is missing", want)
		}
	}

	read := mustCall(t, s, "vault_read", map[string]any{"id": "Notes/Alpha.md"})
	version, _ := read["version"].(string)
	if read["title"] != "Alpha" || version == "" {
		t.Fatalf("vault_read = %v", read)
	}

	edited := mustCall(t, s, "vault_edit", map[string]any{
		"id": "Notes/Alpha.md", "old_text": "About graphs.", "new_text": "About graphs and agents.", "version": version,
	})
	if edited["version"] == version {
		t.Fatal("vault_edit did not change the version")
	}
	if _, errText := call(t, s, "vault_edit", map[string]any{
		"id": "Notes/Alpha.md", "old_text": "agents", "new_text": "x", "version": version,
	}); !strings.HasPrefix(errText, "stale_version") {
		t.Fatalf("edit with an old version: got %q, want stale_version", errText)
	}

	mustCall(t, s, "vault_write", map[string]any{"id": "Notes/Beta.md", "content": "# Beta\nLinks to [[Alpha]].\n"})
	mustCall(t, s, "vault_append", map[string]any{"id": "Notes/Beta.md", "text": "More."})

	found := mustCall(t, s, "vault_search", map[string]any{"query": "agents"})
	results, _ := found["results"].([]any)
	if len(results) != 1 || results[0].(map[string]any)["id"] != "Notes/Alpha.md" {
		t.Fatalf("vault_search = %v", found)
	}

	back := mustCall(t, s, "vault_backlinks", map[string]any{"id": "Notes/Alpha.md"})
	if ids, _ := back["ids"].([]any); len(ids) != 2 {
		t.Fatalf("vault_backlinks = %v, want Home.md and Notes/Beta.md", back)
	}

	list := mustCall(t, s, "vault_list", map[string]any{"folder": "Notes"})
	if notes, _ := list["notes"].([]any); len(notes) != 2 {
		t.Fatalf("vault_list = %v", list)
	}

	if _, errText := call(t, s, "vault_read", map[string]any{"id": "../outside.md"}); !strings.HasPrefix(errText, "outside_vault") {
		t.Fatalf("read outside the vault: got %q", errText)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".brainvault", "audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	log := string(data)
	for _, want := range []string{`"actor":"e2e"`, `"tool":"vault_edit"`, `"outcome":"stale_version"`, `"tool":"vault_search"`, `"results":["Notes/Alpha.md"]`} {
		if !strings.Contains(log, want) {
			t.Errorf("audit log is missing %s", want)
		}
	}
	if strings.Contains(log, "About graphs") {
		t.Error("audit log must not contain note content")
	}
}

func TestStartupFailsWithoutVault(t *testing.T) {
	cmd := exec.Command(binary, "mcp")
	cmd.Env = cleanEnv()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("want a non-zero exit, got %v", err)
	}
	if !strings.Contains(stderr.String(), "BRAINVAULT_PATH") {
		t.Fatalf("stderr = %q, want it to mention BRAINVAULT_PATH", stderr.String())
	}
}
