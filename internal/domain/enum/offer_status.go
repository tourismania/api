package enum

// OfferStatus is the lifecycle state of an Offer.
type OfferStatus string

const (
	// OfferStatusDraft means the offer is only visible to its owning
	// agency (and super admins) — not yet shown to clients.
	OfferStatusDraft OfferStatus = "draft"
	// OfferStatusPublished means the offer is visible to every
	// authenticated client (ROLE_USER).
	OfferStatusPublished OfferStatus = "published"
)

// String implements fmt.Stringer.
func (s OfferStatus) String() string { return string(s) }

// IsValid reports whether s is one of the known offer statuses.
func (s OfferStatus) IsValid() bool {
	switch s {
	case OfferStatusDraft, OfferStatusPublished:
		return true
	default:
		return false
	}
}
