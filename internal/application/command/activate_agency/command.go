// Package activateagency holds the ActivateAgency command, its handler
// and result. Backs the `agency activate` CLI command.
package activateagency

// Command represents the intent to activate an existing agency.
type Command struct {
	ID int
}
