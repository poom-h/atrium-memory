package vault

// MatchIDs lists the IDs of search results, for the audit log. It is never nil.
func MatchIDs(matches []Match) []string {
	ids := make([]string, 0, len(matches))
	for _, m := range matches {
		ids = append(ids, m.ID)
	}
	return ids
}

// IDOr is the stored ID after a successful change, or asked when the change failed.
func (s Saved) IDOr(asked string) string {
	if s.ID != "" {
		return s.ID
	}
	return asked
}

// IDOr is the note's resolved ID, or asked when the read failed.
func (n Note) IDOr(asked string) string {
	if n.ID != "" {
		return n.ID
	}
	return asked
}

// IDOr is a resolved note ID, or asked when resolving failed (resolved is empty).
func IDOr(resolved, asked string) string {
	if resolved != "" {
		return resolved
	}
	return asked
}
