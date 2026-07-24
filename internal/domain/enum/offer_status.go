package enum

// OfferStatus is the lifecycle state of an Offer.
type OfferStatus string

const (
	// OfferStatusDraft means the offer is still being edited — only
	// visible to its owning agency's staff (ROLE_AGENT/ROLE_SUPER_ADMIN
	// of that agency), not yet shown to clients.
	OfferStatusDraft OfferStatus = "draft"
	// OfferStatusReady means the offer's content is complete and saved,
	// but the agency has not yet decided to publish it. Same visibility
	// as OfferStatusDraft — owning agency staff only.
	OfferStatusReady OfferStatus = "ready"
	// OfferStatusPublished means the offer is visible to everyone,
	// including anonymous/unauthenticated callers.
	OfferStatusPublished OfferStatus = "published"
)

// String implements fmt.Stringer.
func (s OfferStatus) String() string { return string(s) }

// IsValid reports whether s is one of the known offer statuses.
func (s OfferStatus) IsValid() bool {
	switch s {
	case OfferStatusDraft, OfferStatusReady, OfferStatusPublished:
		return true
	default:
		return false
	}
}
