package cli

import (
	"errors"
	"fmt"

	activateagency "api/internal/application/command/activate_agency"
	createagency "api/internal/application/command/create_agency"
	deactivateagency "api/internal/application/command/deactivate_agency"

	"github.com/spf13/cobra"
)

// NewAgencyCommand returns the `agency` cobra command group.
//
// Usage:
//
//	app agency create --name "<name>"
//	app agency deactivate --id <id>
//	app agency activate --id <id>
func NewAgencyCommand(
	createUC createagency.UseCase,
	deactivateUC deactivateagency.UseCase,
	activateUC activateagency.UseCase,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agency",
		Short: "Manage travel agencies",
	}
	cmd.AddCommand(newAgencyCreateCommand(createUC))
	cmd.AddCommand(newAgencyDeactivateCommand(deactivateUC))
	cmd.AddCommand(newAgencyActivateCommand(activateUC))
	return cmd
}

func newAgencyCreateCommand(uc createagency.UseCase) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new agency",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return errors.New("--name is required")
			}
			res, err := uc.Handle(cmd.Context(), createagency.Command{Name: name})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Agency successfully created! id=%d uuid=%s\n", res.ID, res.UUID)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Agency name")
	return cmd
}

func newAgencyDeactivateCommand(uc deactivateagency.UseCase) *cobra.Command {
	var id int

	cmd := &cobra.Command{
		Use:   "deactivate",
		Short: "Deactivate an existing agency",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if id <= 0 {
				return errors.New("--id is required")
			}
			if _, err := uc.Handle(cmd.Context(), deactivateagency.Command{ID: id}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Agency successfully deactivated! id=%d\n", id)
			return nil
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "Agency id")
	return cmd
}

func newAgencyActivateCommand(uc activateagency.UseCase) *cobra.Command {
	var id int

	cmd := &cobra.Command{
		Use:   "activate",
		Short: "Activate an existing agency",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if id <= 0 {
				return errors.New("--id is required")
			}
			if _, err := uc.Handle(cmd.Context(), activateagency.Command{ID: id}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Agency successfully activated! id=%d\n", id)
			return nil
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "Agency id")
	return cmd
}
