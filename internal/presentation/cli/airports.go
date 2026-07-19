package cli

import (
	syncairports "api/internal/application/command/sync_airports"

	"github.com/spf13/cobra"
)

// NewAirportsCommand returns the `airports` cobra command group.
//
// Usage:
//
//	app airports sync [--dry-run]
func NewAirportsCommand(syncUC syncairports.UseCase) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "airports",
		Short: "Manage the airports/cities/countries reference data",
	}
	cmd.AddCommand(newAirportsSyncCommand(syncUC))
	return cmd
}

func newAirportsSyncCommand(uc syncairports.UseCase) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "sync",
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
