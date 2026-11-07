package vault

import (
	"slices"
	"strings"
	"testing"
)

func searchVault(t *testing.T) *Vault {
	return newVault(t, map[string]string{
		"Notes/LangGraph basics.md": "# LangGraph basics\nGraphs of agents.",
		"Notes/Agents.md":           "Notes about langgraph and other tools.",
		"Maps/Tools.md":             "---\ntags: [langgraph]\n---\nNothing here.",
		"Archive/Old langgraph.md":  "old",
	})
}

func matchIDs(ms []Match) []string {
	out := []string{}
	for _, m := range ms {
		out = append(out, m.ID)
	}
	return out
}

func TestSearchRanking(t *testing.T) {
	v := searchVault(t)
	got, err := v.Search("LangGraph", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	// Title prefix (720) > title contains (520) > body only (4). Properties are not searched.
	want := []string{"Notes/LangGraph basics.md", "Archive/Old langgraph.md", "Notes/Agents.md"}
	if ids := matchIDs(got); !slices.Equal(ids, want) {
		t.Fatalf("got %v, want %v", ids, want)
	}
}

func TestSearchNeedsEveryWord(t *testing.T) {
	v := searchVault(t)
	got, _ := v.Search("langgraph tools", "", 20)
	if ids := matchIDs(got); !slices.Equal(ids, []string{"Notes/Agents.md"}) {
		t.Fatalf("got %v", ids)
	}
}

func TestSearchFolderLimitAndEmpty(t *testing.T) {
	v := searchVault(t)
	got, _ := v.Search("langgraph", "Notes", 20)
	if ids := matchIDs(got); !slices.Equal(ids, []string{"Notes/LangGraph basics.md", "Notes/Agents.md"}) {
		t.Fatalf("folder: got %v", ids)
	}
	if got, _ := v.Search("langgraph", "", 1); len(got) != 1 {
		t.Fatalf("limit: got %d results", len(got))
	}
	got, err := v.Search("   ", "", 20)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty query: got %v, %v; want an empty non-nil list", got, err)
	}
}

func TestSnippet(t *testing.T) {
	body := strings.Repeat("a ", 60) + "needle" + strings.Repeat(" b", 60)
	s := snippet([]string{"needle"}, body, 90)
	if !strings.HasPrefix(s, "…") || !strings.HasSuffix(s, "…") || !strings.Contains(s, "needle") {
		t.Fatalf("snippet = %q", s)
	}
	if s := snippet([]string{"zzz"}, "short   text", 90); s != "short text" {
		t.Fatalf("no-hit snippet = %q", s)
	}
}

func TestReadable(t *testing.T) {
	got := readable("# Title\n- see [[Other|the other note]] and [docs](https://x.y)\n**bold** `code`", "Title")
	if got != "see the other note and docs\nbold code" {
		t.Fatalf("readable = %q", got)
	}
}
