package agency

// Status is the lifecycle state of a travel agency.
type Status string

const (
	// StatusActive means the agency may own offers and receive new
	// registered agents.
	StatusActive Status = "active"
	// StatusInactive means the agency is deactivated: existing data
	// is retained but no new offers/agents may be attached to it.
	StatusInactive Status = "inactive"
)

// String implements fmt.Stringer.
func (s Status) String() string { return string(s) }
