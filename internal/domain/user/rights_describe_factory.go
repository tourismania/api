package user

// RightsDescribeFactory is stateless; expose as a struct (not a free
// function) so consumers can wire it through DI alongside other services
// — same shape as the PHP RightsDescribeFactory.
type RightsDescribeFactory struct{}

// NewRightsDescribeFactory is the canonical constructor.
func NewRightsDescribeFactory() *RightsDescribeFactory {
	return &RightsDescribeFactory{}
}

// ByRoles inspects the supplied role list and lights up the RightsDescribe
// flags accordingly. Unknown roles are ignored silently — they don't
// contribute to any flag.
func (f *RightsDescribeFactory) ByRoles(roles []string) RightsDescribe {
	isSuperAdmin := false
	for _, r := range roles {
		if r == string(RoleSuperAdmin) {
			isSuperAdmin = true
			break
		}
	}
	return RightsDescribe{IsSuperAdmin: isSuperAdmin}
}
