package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/arcgolabs/dix"
	"github.com/spf13/cobra"
)

func main() {
	var configFile string

	cmd := &cobra.Command{
		Use:           "orivis-server",
		Short:         "Run the Orivis server",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			app := newServerApp(ctx, cmd, configFile)
			if err := app.RunContext(ctx); err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("run server app: %w", err)
			}
			return nil
		},
	}

	registerServerFlags(cmd, &configFile)

	if err := cmd.Execute(); err != nil {
		if _, writeErr := fmt.Fprintln(os.Stderr, err); writeErr != nil {
			os.Exit(1)
		}
		os.Exit(1)
	}
}

func newServerApp(ctx context.Context, cmd *cobra.Command, configFile string) *dix.App {
	return dix.New("orivis-server",
		dix.WithProfile(dix.ProfileDev),
		dix.WithVersion(buildServerVersion()),
		dix.WithAppDescription("distributed availability observability platform"),
		dix.WithRunStopTimeout(defaultServerRunStopTimeout()),
		dix.WithModules(newServerModules(ctx, cmd, configFile)...),
	)
}
