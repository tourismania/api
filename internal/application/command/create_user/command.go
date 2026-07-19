// Package createuser holds the CreateUser command, its handler and result.
package createuser

// Command represents the intent to register a new user. All fields are
// already validated at the presentation boundary; this struct is the
// application-layer DTO.
type Command struct {
	FirstName string
	LastName  string
	Email     string
	Password  string
	// AgencyID links the new user to an existing, active agency
	// (1 user = 1 agency). Required for every registration.
	AgencyID int
}
