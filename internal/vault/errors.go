// Package vault reads, searches and safely changes a Markdown vault. Every rule in SPEC.md lives here.
package vault

import (
	"errors"
	"fmt"
)

// Code is a machine-readable failure reason that tools return to agents.
type Code string

const (
	NotFound       Code = "not_found"
	Exists         Code = "exists"
	OutsideVault   Code = "outside_vault"
	StaleVersion   Code = "stale_version"
	NoMatch        Code = "no_match"
	AmbiguousMatch Code = "ambiguous_match"
	InvalidNote    Code = "invalid_note"
	Locked         Code = "locked"
	Internal       Code = "internal"
)

// Error is a vault failure an agent can act on.
type Error struct {
	Code           Code
	Message        string
	CurrentVersion string // set for stale_version and exists, so the agent can re-read and retry
}

func (e *Error) Error() string {
	s := fmt.Sprintf("%s: %s", e.Code, e.Message)
	if e.CurrentVersion != "" {
		s += fmt.Sprintf(" (current_version: %s)", e.CurrentVersion)
	}
	return s
}

func errorf(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// CodeOf returns the Code of a vault error, or Internal for any other error.
func CodeOf(err error) Code {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return Internal
}

// OutcomeOf is the audit-log outcome for a call: "ok", or the error's code.
func OutcomeOf(err error) string {
	if err == nil {
		return "ok"
	}
	return string(CodeOf(err))
}
