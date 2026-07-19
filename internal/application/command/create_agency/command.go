// Package createagency holds the CreateAgency command, its handler and
// result. Backs the `agency create` CLI command — there is no HTTP CRUD
// for agencies in this iteration.
package createagency

// Command represents the intent to register a new agency.
type Command struct {
	Name string
}
