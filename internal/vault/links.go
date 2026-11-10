package vault

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

var wikilink = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

// linkIndex finds the note a wikilink names, following the rules in SPEC.md.
type linkIndex struct {
	byStem map[string][]string // lowercased title -> IDs, shortest first
	byPath map[string]string   // lowercased ID without .md -> ID
}

func newLinkIndex(entries []Entry) linkIndex {
	li := linkIndex{byStem: map[string][]string{}, byPath: map[string]string{}}
	for _, e := range entries {
		li.byStem[strings.ToLower(e.Title)] = append(li.byStem[strings.ToLower(e.Title)], e.ID)
		li.byPath[strings.ToLower(strings.TrimSuffix(e.ID, ".md"))] = e.ID
	}
	for _, ids := range li.byStem {
		sort.Slice(ids, func(i, j int) bool {
			if len(ids[i]) != len(ids[j]) {
				return len(ids[i]) < len(ids[j])
			}
			return ids[i] < ids[j]
		})
	}
	return li
}

// resolve returns the ID a wikilink target means, or "" when no note matches.
func (li linkIndex) resolve(target string) string {
	name := target
	if i := strings.Index(name, "|"); i >= 0 {
		name = name[:i]
	}
	if i := strings.Index(name, "#"); i >= 0 {
		name = name[:i]
	}
	key := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(norm.NFC.String(name))), ".md")
	if strings.Contains(key, "/") {
		if id, ok := li.byPath[key]; ok {
			return id
		}
		key = key[strings.LastIndex(key, "/")+1:]
	}
	if ids := li.byStem[key]; len(ids) > 0 {
		return ids[0]
	}
	return ""
}

// outgoing returns the notes a text links to, in order, each once.
func (li linkIndex) outgoing(text string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, m := range wikilink.FindAllStringSubmatch(text, -1) {
		if id := li.resolve(m[1]); id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// Backlinks returns the notes, other than the note itself, that link to it, and the note's ID as stored
// (empty when the note wasn't found), for the audit log.
func (v *Vault) Backlinks(id string) (ids []string, resolved string, err error) {
	t, err := v.resolve(id)
	if err != nil {
		return nil, "", err
	}
	if !t.Exists {
		return nil, "", errorf(NotFound, "no note %q", id)
	}
	entries, err := v.notes()
	if err != nil {
		return nil, t.ID, err
	}
	li := newLinkIndex(entries)
	out := []string{}
	for _, e := range entries {
		if e.ID == t.ID {
			continue
		}
		data, err := os.ReadFile(e.abs)
		if err != nil || !utf8.Valid(data) {
			continue
		}
		for _, linked := range li.outgoing(string(data)) {
			if linked == t.ID {
				out = append(out, e.ID)
				break
			}
		}
	}
	return out, t.ID, nil
}
