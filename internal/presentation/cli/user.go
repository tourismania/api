// Package cli holds cobra command definitions. They share the same
// application use-cases as the HTTP layer — no duplicated orchestration.
package cli

import (
	"errors"
	"fmt"

	createuser "api/internal/application/command/create_user"

	"github.com/spf13/cobra"
)

// NewUserCommand returns the `user` cobra command group.
//
// Usage:
//
//	app user create <firstName> <lastName> <email> <password>
func NewUserCommand(createUC createuser.UseCase) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Manage users",
	}
	cmd.AddCommand(newUserCreateCommand(createUC))
	return cmd
}

func newUserCreateCommand(uc createuser.UseCase) *cobra.Command {
	var agencyID int

	cmd := &cobra.Command{
		Use:   "create <firstName> <lastName> <email> <password>",
		Short: "Create a new user",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			firstName, lastName, email, password := args[0], args[1], args[2], args[3]
			if email == "" || password == "" {
				return errors.New("email and password are required")
			}
			if agencyID <= 0 {
				return errors.New("--agency-id is required")
			}
			res, err := uc.Handle(cmd.Context(), createuser.Command{
				FirstName: firstName,
				LastName:  lastName,
				Email:     email,
				Password:  password,
				AgencyID:  agencyID,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "User successfully generated! id=%d\n", res.ID)
			return nil
		},
	}
	cmd.Flags().IntVar(&agencyID, "agency-id", 0, "Agency id (required)")
	return cmd
}
