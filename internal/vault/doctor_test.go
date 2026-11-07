package vault

import (
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	v := newVault(t, map[string]string{
		"Bad.md":         "---\nkey: [unclosed\n---\nbody\n",
		"Ok.md":          "[[Nowhere]] [[Alpha]]\n",
		"Alpha.md":       "",
		"Notes/Alpha.md": "",
		"Binary.md":      "\xff",
	})
	r, err := v.Check()
	if err != nil {
		t.Fatal(err)
	}
	if r.Notes != 5 {
		t.Fatalf("notes = %d, want 5", r.Notes)
	}
	joinedErrors := strings.Join(r.Errors, "\n")
	for _, want := range []string{"Bad.md: properties block", "Binary.md: not valid UTF-8"} {
		if !strings.Contains(joinedErrors, want) {
			t.Errorf("errors missing %q:\n%s", want, joinedErrors)
		}
	}
	joinedWarnings := strings.Join(r.Warnings, "\n")
	for _, want := range []string{"Ok.md: link [[Nowhere]] has no matching note", `notes share the name "alpha"`} {
		if !strings.Contains(joinedWarnings, want) {
			t.Errorf("warnings missing %q:\n%s", want, joinedWarnings)
		}
	}
}
