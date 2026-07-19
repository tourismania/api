// Package main is the CLI entrypoint. Hosts cobra commands that share
// the same composition root with the HTTP server.
package main

import (
	"context"
	"log/slog"
	"os"

	"api/config"
	clicmd "api/internal/presentation/cli"

	"github.com/spf13/cobra"
)

func main() {
	if err := run(); err != nil {
		slog.Error("cli fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx := context.Background()
	container, err := config.Build(ctx, cfg)
	if err != nil {
		return err
	}
	defer container.Close()

	root := &cobra.Command{
		Use:           "app",
		Short:         "Tourismania CLI",
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	root.AddCommand(clicmd.NewUserCommand(container.App.CreateUser))
	root.AddCommand(clicmd.NewAirportsCommand(container.App.SyncAirports))
	root.AddCommand(clicmd.NewAgencyCommand(container.App.CreateAgency, container.App.DeactivateAgency, container.App.ActivateAgency))
	return root.Execute()
}
