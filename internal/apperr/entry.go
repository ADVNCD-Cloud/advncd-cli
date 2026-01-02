package apperr

// Entry is a catalog item: a stable error code + user-facing message.
// This is the single source of truth (no separate const + map keys duplication).
type Entry struct {
	Code    string
	Message string
}

// E is a small helper to define entries consistently.
func E(code, message string) Entry {
	return Entry{Code: code, Message: message}
}