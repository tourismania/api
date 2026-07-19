package enum

// AgencyStatus is the lifecycle state of a travel agency.
type AgencyStatus string

const (
	// AgencyStatusActive means the agency may own offers and receive new
	// registered agents.
	AgencyStatusActive AgencyStatus = "active"
	// AgencyStatusInactive means the agency is deactivated: existing data
	// is retained but no new offers/agents may be attached to it.
	AgencyStatusInactive AgencyStatus = "inactive"
)

// String implements fmt.Stringer.
func (s AgencyStatus) String() string { return string(s) }
