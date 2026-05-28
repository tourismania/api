package cli

import (
	syncairports "api/internal/application/command/sync_airports"

	"github.com/spf13/cobra"
)

// NewSyncAirportsCommand returns the `sync-airports` cobra command.
//
// Usage:
//
//	app sync-airports [--dry-run]
func NewSyncAirportsCommand(uc syncairports.UseCase) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "sync-airports",
		Short: "Sync airports, cities, and countries from external sources (mwgg + Wikidata)",
		Long: `Downloads the full airports dataset from github.com/mwgg/Airports and
enriches it with Russian-language names via Wikidata SPARQL.
Upserts all countries, cities, and airports into the database.

Intended to be run once a month as a scheduled admin task.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := uc.Handle(cmd.Context(), syncairports.Command{
				DryRun:   dryRun,
				Progress: cmd.OutOrStdout(),
			})
			if err != nil {
				return err
			}
			if dryRun {
				cmd.Printf("[dry-run] countries=%d  cities=%d  airports=%d\n",
					res.Countries, res.Cities, res.Airports)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print what would be synced without writing to the database")
	return cmd
}
