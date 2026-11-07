// Package audit appends one JSON line per tool call to <vault>/.brainvault/audit.jsonl.
// Entries never contain note content.
package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Entry is one logged call.
type Entry struct {
	Time    string   `json:"time"`
	Actor   string   `json:"actor"`
	Tool    string   `json:"tool"`
	ID      string   `json:"id,omitempty"`
	Folder  string   `json:"folder,omitempty"`
	Query   string   `json:"query,omitempty"`
	Results []string `json:"results,omitempty"`
	Outcome string   `json:"outcome"`
}

// Log appends entries to a vault's audit file.
type Log struct {
	path string
	mu   sync.Mutex
	now  func() time.Time
}

// New returns the log for the vault rooted at vaultRoot.
func New(vaultRoot string) *Log {
	return &Log{path: filepath.Join(vaultRoot, ".brainvault", "audit.jsonl"), now: time.Now}
}

// Path is the audit file's location.
func (l *Log) Path() string { return l.path }

// Record appends one entry, filling in Time when it is empty.
func (l *Log) Record(e Entry) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e.Time == "" {
		e.Time = l.now().UTC().Format(time.RFC3339Nano)
	}
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}
