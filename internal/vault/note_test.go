package vault

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestRead(t *testing.T) {
	body := "---\ntags: [ai, notes]\ncreated: 2026-01-02\n---\n# Alpha\nText.\n"
	v := newVault(t, map[string]string{"Notes/Alpha.md": body})
	n, err := v.Read("notes/alpha.md")
	if err != nil {
		t.Fatal(err)
	}
	if n.ID != "Notes/Alpha.md" || n.Title != "Alpha" || n.Content != body {
		t.Fatalf("got %+v", n)
	}
	sum := sha256.Sum256([]byte(body))
	if n.Version != hex.EncodeToString(sum[:]) {
		t.Fatalf("version = %s", n.Version)
	}
	if tags, _ := n.Frontmatter["tags"].([]any); len(tags) != 2 || tags[0] != "ai" {
		t.Fatalf("tags = %#v", n.Frontmatter["tags"])
	}
	if created, _ := n.Frontmatter["created"].(string); !strings.HasPrefix(created, "2026-01-02") {
		t.Fatalf("created = %#v, want a string starting 2026-01-02", n.Frontmatter["created"])
	}
	if n.Links == nil {
		t.Fatal("links must be an empty list, not nil")
	}
}

func TestReadErrors(t *testing.T) {
	v := newVault(t, map[string]string{"Bad.md": "\xff\xfe"})
	_, err := v.Read("Missing.md")
	wantCode(t, err, NotFound)
	_, err = v.Read("Bad.md")
	wantCode(t, err, InvalidNote)
}

func TestFrontmatter(t *testing.T) {
	cases := map[string]bool{
		"no properties\n":                  false,
		"---\n---\nempty block\n":          false,
		"---\nkey: [unclosed\n---\nbody\n": false,
		"---\r\nkey: value\r\n---\r\nbody": true,
	}
	for text, want := range cases {
		if got := parseFrontmatter(text) != nil; got != want {
			t.Errorf("parseFrontmatter(%q) present = %v, want %v", text, got, want)
		}
	}
	_, body, ok := splitFrontmatter("---\na: 1\n---\nHello\n")
	if !ok || body != "Hello\n" {
		t.Fatalf("body = %q ok = %v", body, ok)
	}
}

func TestList(t *testing.T) {
	v := newVault(t, map[string]string{
		"Home.md": "", "Notes/A.md": "", "Notes/Deep/B.md": "",
		".obsidian/x.md": "", "Notes/.hidden.md": "", "Notes/readme.txt": "",
	})
	ids := func(es []Entry) []string {
		out := []string{}
		for _, e := range es {
			out = append(out, e.ID)
		}
		return out
	}
	all, err := v.List("", true)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(all); !slices.Equal(got, []string{"Home.md", "Notes/A.md", "Notes/Deep/B.md"}) {
		t.Fatalf("all = %v", got)
	}
	if _, err := time.Parse(time.RFC3339, all[0].Modified); err != nil {
		t.Fatalf("modified %q is not RFC 3339", all[0].Modified)
	}
	notes, _ := v.List("notes/", true)
	if got := ids(notes); !slices.Equal(got, []string{"Notes/A.md", "Notes/Deep/B.md"}) {
		t.Fatalf("notes = %v", got)
	}
	flat, _ := v.List("Notes", false)
	if got := ids(flat); !slices.Equal(got, []string{"Notes/A.md"}) {
		t.Fatalf("flat = %v", got)
	}
	_, err = v.List("../x", true)
	wantCode(t, err, OutsideVault)
}
