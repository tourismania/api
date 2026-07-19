// Package deactivateagency holds the DeactivateAgency command, its
// handler and result. Backs the `agency deactivate` CLI command.
package deactivateagency

// Command represents the intent to deactivate an existing agency.
type Command struct {
	ID int
}
