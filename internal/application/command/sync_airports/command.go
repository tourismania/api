// Package syncairports contains the sync-airports command and its handler.
package syncairports

import "io"

// Command carries the parameters for the sync-airports use-case.
type Command struct {
	// DryRun prints what would be done without writing to the database.
	DryRun bool

	// Progress, if non-nil, receives human-readable progress lines during sync.
	Progress io.Writer
}
