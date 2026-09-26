package user

// RightsDescriber is a thin domain service that wraps the factory so
// consumers (e.g. GetMe handler) don't need to instantiate the factory
// themselves and the dependency stays explicit in DI.
type RightsDescriber struct {
	factory *RightsDescribeFactory
}

// NewRightsDescriber constructs the service.
func NewRightsDescriber(f *RightsDescribeFactory) *RightsDescriber {
	return &RightsDescriber{factory: f}
}

// ByRoles delegates to the underlying factory; kept on a separate type so
// future audit/logging hooks can attach without changing the factory.
func (s *RightsDescriber) ByRoles(roles []string) RightsDescribe {
	return s.factory.ByRoles(roles)
}
