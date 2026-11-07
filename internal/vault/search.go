package vault

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// Match is one search result.
type Match struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	score   int
}

const (
	defaultLimit = 20
	maxLimit     = 100
	snippetWidth = 90
)

// Search finds notes where every word of query appears in the title, path or text (properties excluded).
// Results are ranked like ClaudeDesk's Cockpit search: title matches first, then path, then text.
func (v *Vault) Search(query, folder string, limit int) ([]Match, error) {
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	out := []Match{}
	phrase := strings.ToLower(strings.TrimSpace(query))
	terms := strings.Fields(phrase)
	if len(terms) == 0 {
		return out, nil
	}
	entries, err := v.List(folder, true)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		data, err := os.ReadFile(e.abs)
		if err != nil || !utf8.Valid(data) {
			continue
		}
		_, body, _ := splitFrontmatter(string(data))
		if m, ok := score(e, readable(body, e.Title), phrase, terms); ok {
			out = append(out, m)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func score(e Entry, body, phrase string, terms []string) (Match, bool) {
	title, p, text := strings.ToLower(e.Title), strings.ToLower(e.ID), strings.ToLower(body)
	for _, t := range terms {
		if !strings.Contains(title, t) && !strings.Contains(p, t) && !strings.Contains(text, t) {
			return Match{}, false
		}
	}
	s := 0
	switch {
	case title == phrase:
		s += 1000
	case strings.HasPrefix(title, phrase):
		s += 600
	case strings.Contains(title, phrase):
		s += 400
	}
	for _, t := range terms {
		if strings.Contains(title, t) {
			if strings.HasPrefix(title, t) || strings.Contains(title, " "+t) {
				s += 120
			} else {
				s += 80
			}
		} else if strings.Contains(p, t) {
			s += 40
		}
		s += min(20, strings.Count(text, t)*4)
	}
	return Match{ID: e.ID, Title: e.Title, Snippet: snippet(terms, body, snippetWidth), score: s}, true
}

var (
	reWikilinkLabel = regexp.MustCompile(`\[\[([^\]|]*\|)?([^\]]*)\]\]`)
	reMarkdownLink  = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	reLineMarker    = regexp.MustCompile(`(?m)^\s*(#{1,6}|>|[-*+]|\d+\.)\s+`)
	reEmphasis      = regexp.MustCompile("[*`~]")
	reSpace         = regexp.MustCompile(`\s+`)
)

// readable is a note's text as it reads: no Markdown marks, wikilinks as their label, and no opening
// heading that only repeats the title. Underscores stay, since they are usually part of names.
func readable(markdown, title string) string {
	lines := strings.Split(markdown, "\n")
	for i, l := range lines {
		tl := strings.TrimSpace(l)
		if tl == "" {
			continue
		}
		if strings.HasPrefix(tl, "#") && strings.EqualFold(strings.Trim(tl, "# "), title) {
			lines = append(lines[:i], lines[i+1:]...)
		}
		break
	}
	text := strings.Join(lines, "\n")
	text = reWikilinkLabel.ReplaceAllString(text, "$2")
	text = reMarkdownLink.ReplaceAllString(text, "$1")
	text = reLineMarker.ReplaceAllString(text, "")
	return reEmphasis.ReplaceAllString(text, "")
}

// snippet is about width characters around the first matching word, on one line, with ellipses where cut.
func snippet(terms []string, body string, width int) string {
	flat := []rune(strings.TrimSpace(reSpace.ReplaceAllString(body, " ")))
	lower := strings.ToLower(string(flat))
	hit := -1
	if utf8.RuneCountInString(lower) == len(flat) { // offsets only map back when lowercasing kept the length
		for _, t := range terms {
			if i := strings.Index(lower, t); i >= 0 {
				if r := utf8.RuneCountInString(lower[:i]); hit < 0 || r < hit {
					hit = r
				}
			}
		}
	}
	if hit < 0 {
		return string(flat[:min(width, len(flat))])
	}
	start := max(0, hit-width/3)
	end := min(len(flat), start+width)
	s := string(flat[start:end])
	if start > 0 {
		s = "…" + s
	}
	if end < len(flat) {
		s += "…"
	}
	return s
}
