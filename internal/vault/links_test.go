package vault

import (
	"slices"
	"testing"
)

func TestReadLinks(t *testing.T) {
	v := newVault(t, map[string]string{
		"Home.md":        "[[Alpha]] [[alpha|label]] [[Alpha#Part]] [[Notes/Beta]] [[Beta.md]] [[Other/Alpha]] [[Missing]]",
		"Alpha.md":       "",
		"Notes/Alpha.md": "",
		"Notes/Beta.md":  "",
	})
	n, err := v.Read("Home.md")
	if err != nil {
		t.Fatal(err)
	}
	// Shortest ID wins for [[Alpha]]; [[Other/Alpha]] falls back to the name; duplicates appear once.
	if want := []string{"Alpha.md", "Notes/Beta.md"}; !slices.Equal(n.Links, want) {
		t.Fatalf("links = %v, want %v", n.Links, want)
	}
}

func TestLinkWithExactPath(t *testing.T) {
	v := newVault(t, map[string]string{"Home.md": "[[Notes/Alpha]]", "Alpha.md": "", "Notes/Alpha.md": ""})
	n, err := v.Read("Home.md")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Notes/Alpha.md"}; !slices.Equal(n.Links, want) {
		t.Fatalf("links = %v, want %v", n.Links, want)
	}
}

func TestBacklinks(t *testing.T) {
	v := newVault(t, map[string]string{
		"Home.md":      "[[Alpha]]",
		"Notes/Map.md": "see [[Alpha|it]] and [[Alpha]]",
		"Alpha.md":     "[[Alpha]] links to itself",
		"Other.md":     "no links",
	})
	got, resolved, err := v.Backlinks("alpha.md")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != "Alpha.md" {
		t.Fatalf("resolved ID = %q, want Alpha.md", resolved)
	}
	if want := []string{"Home.md", "Notes/Map.md"}; !slices.Equal(got, want) {
		t.Fatalf("backlinks = %v, want %v", got, want)
	}
	_, _, err = v.Backlinks("Nope.md")
	wantCode(t, err, NotFound)
}
