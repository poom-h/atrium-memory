package vault

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// Note is one note as tools return it.
type Note struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Content     string         `json:"content"`
	Frontmatter map[string]any `json:"frontmatter,omitempty"`
	Links       []string       `json:"links"`
	Version     string         `json:"version"`
}

// Read returns a note with its parsed properties, outgoing links and version.
func (v *Vault) Read(id string) (Note, error) {
	t, err := v.resolve(id)
	if err != nil {
		return Note{}, err
	}
	if !t.Exists {
		return Note{}, errorf(NotFound, "no note %q; use vault_search or vault_list to find notes", id)
	}
	data, err := readNote(t)
	if err != nil {
		return Note{}, err
	}
	text := string(data)
	entries, err := v.notes()
	if err != nil {
		return Note{}, err
	}
	return Note{
		ID:          t.ID,
		Title:       titleOf(t.ID),
		Content:     text,
		Frontmatter: parseFrontmatter(text),
		Links:       newLinkIndex(entries).outgoing(text),
		Version:     versionOf(data),
	}, nil
}

// readNote reads an existing note's bytes and checks they are UTF-8.
func readNote(t target) ([]byte, error) {
	data, err := os.ReadFile(t.Abs)
	if err != nil {
		return nil, errorf(Internal, "cannot read %q: %s", t.ID, cause(err))
	}
	if !utf8.Valid(data) {
		return nil, errorf(InvalidNote, "%q is not valid UTF-8 text", t.ID)
	}
	return data, nil
}

func versionOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func titleOf(id string) string {
	return strings.TrimSuffix(path.Base(id), path.Ext(id))
}

// splitFrontmatter returns the YAML properties block (without its fences) and the body after it,
// with line endings converted to LF. ok is false when the note has no properties block.
func splitFrontmatter(text string) (yamlText, body string, ok bool) {
	lf := strings.ReplaceAll(text, "\r\n", "\n")
	if !strings.HasPrefix(lf, "---\n") {
		return "", lf, false
	}
	rest := lf[len("---\n"):]
	if rest == "---" || strings.HasPrefix(rest, "---\n") {
		return "", strings.TrimPrefix(strings.TrimPrefix(rest, "---"), "\n"), true
	}
	search := 0
	for {
		i := strings.Index(rest[search:], "\n---")
		if i < 0 {
			return "", lf, false
		}
		end := search + i
		after := rest[end+len("\n---"):]
		if after == "" || strings.HasPrefix(after, "\n") {
			return rest[:end], strings.TrimPrefix(after, "\n"), true
		}
		search = end + 1
	}
}

// parseFrontmatter returns a note's properties as plain JSON values, or nil if it has none or they don't parse.
func parseFrontmatter(text string) map[string]any {
	y, _, ok := splitFrontmatter(text)
	if !ok || strings.TrimSpace(y) == "" {
		return nil
	}
	var raw map[string]any
	if err := yaml.Unmarshal([]byte(y), &raw); err != nil || len(raw) == 0 {
		return nil
	}
	// Round-trip through JSON so every value is a plain JSON type (dates become strings).
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return out
}
