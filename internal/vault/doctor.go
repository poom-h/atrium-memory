package vault

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// Report is the result of checking a vault against SPEC.md.
type Report struct {
	Notes    int      `json:"notes"`
	Errors   []string `json:"errors"`   // notes tools can't use correctly
	Warnings []string `json:"warnings"` // allowed, but probably not intended
}

// Check validates every note: readable (and reports every note or folder it can't read), UTF-8, a properties block that parses, links that resolve, unique names.
func (v *Vault) Check() (Report, error) {
	entries, skipped, err := v.walk()
	if err != nil {
		return Report{}, err
	}
	li := newLinkIndex(entries)
	r := Report{Notes: len(entries), Errors: []string{}, Warnings: []string{}}
	for _, u := range skipped {
		r.Errors = append(r.Errors, fmt.Sprintf("%s: cannot read: %v", u.ID, u.Err))
	}
	for _, e := range entries {
		data, err := os.ReadFile(e.abs)
		if err != nil {
			r.Errors = append(r.Errors, fmt.Sprintf("%s: cannot read: %v", e.ID, err))
			continue
		}
		if !utf8.Valid(data) {
			r.Errors = append(r.Errors, fmt.Sprintf("%s: not valid UTF-8", e.ID))
			continue
		}
		text := string(data)
		if err := frontmatterProblem(text); err != nil {
			r.Errors = append(r.Errors, fmt.Sprintf("%s: properties block: %v", e.ID, err))
		}
		for _, m := range wikilink.FindAllStringSubmatch(text, -1) {
			if li.resolve(m[1]) == "" {
				r.Warnings = append(r.Warnings, fmt.Sprintf("%s: link [[%s]] has no matching note", e.ID, m[1]))
			}
		}
	}
	byFold := map[string][]string{}
	for _, e := range entries {
		k := strings.ToLower(e.ID)
		byFold[k] = append(byFold[k], e.ID)
	}
	for _, ids := range byFold {
		if len(ids) > 1 {
			sort.Strings(ids)
			r.Errors = append(r.Errors, fmt.Sprintf("notes whose IDs differ only in case: %s; SPEC.md doesn't allow this, so tools can't tell them apart", strings.Join(ids, ", ")))
		}
	}
	for stem, ids := range li.byStem {
		if len(ids) > 1 {
			r.Warnings = append(r.Warnings, fmt.Sprintf("notes share the name %q: %s; links by name go to %s", stem, strings.Join(ids, ", "), ids[0]))
		}
	}
	sort.Strings(r.Errors)
	sort.Strings(r.Warnings)
	return r, nil
}

// frontmatterProblem reports why a note's properties block doesn't parse, or nil.
func frontmatterProblem(text string) error {
	y, _, ok := splitFrontmatter(text)
	if !ok || strings.TrimSpace(y) == "" {
		return nil
	}
	var raw map[string]any
	return yaml.Unmarshal([]byte(y), &raw)
}
