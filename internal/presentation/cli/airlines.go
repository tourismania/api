package cli

import (
	"fmt"

	syncairlines "api/internal/application/command/sync_airlines"

	"github.com/spf13/cobra"
)

// NewAirlinesCommand возвращает группу команд `airlines`.
//
// Использование:
//
//	app airlines sync [--dry-run]
func NewAirlinesCommand(syncUC syncairlines.UseCase) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "airlines",
		Short: "Manage the airlines reference data",
	}
	cmd.AddCommand(newAirlinesSyncCommand(syncUC))
	return cmd
}

func newAirlinesSyncCommand(uc syncairlines.UseCase) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync airlines and their countries from Wikidata",
		Long: `Downloads every Wikidata item with an airline ICAO code (P230): ICAO and
IATA codes, English and Russian labels, country and dissolution date.
Upserts countries and airlines by Wikidata QID keeping airline ids; items
gone from Wikidata are hidden from search (merged duplicates or missing),
never deleted. Source anomalies are logged as JSON warnings to stderr
(see docs/cli/sync_airlines.md, "Мониторинг").

Intended to be run once a month as a manual admin task.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := uc.Handle(cmd.Context(), syncairlines.Command{
				DryRun:   dryRun,
				Progress: cmd.OutOrStdout(),
			})
			if err != nil {
				return err
			}
			if dryRun {
				// В stdout, как и прогресс: stderr — только JSON-журнал аномалий.
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] countries=%d  airlines=%d  dissolved=%d  skipped=%d  anomalies=%d\n",
					res.Countries, res.Airlines, res.Dissolved, res.Skipped, res.AnomaliesTotal())
				return err
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Fetch, parse and log anomalies without writing to the database")
	return cmd
}
