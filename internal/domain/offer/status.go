package offer

// Status is the lifecycle state of an Offer.
type Status string

const (
	// StatusDraft means the offer is still being edited — only
	// visible to its owning agency's staff (ROLE_AGENT/ROLE_SUPER_ADMIN
	// of that agency), not yet shown to clients.
	StatusDraft Status = "draft"
	// StatusReady means the offer's content is complete and saved,
	// but the agency has not yet decided to publish it. Same visibility
	// as StatusDraft — owning agency staff only.
	StatusReady Status = "ready"
	// StatusPublished means the offer is visible to everyone,
	// including anonymous/unauthenticated callers.
	StatusPublished Status = "published"
)

// String implements fmt.Stringer.
func (s Status) String() string { return string(s) }

// IsValid reports whether s is one of the known offer statuses.
func (s Status) IsValid() bool {
	switch s {
	case StatusDraft, StatusReady, StatusPublished:
		return true
	default:
		return false
	}
}
