package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	serverapp "github.com/lyonbrown4d/orivis/internal/server/app"
	"github.com/lyonbrown4d/orivis/internal/server/config"
	"github.com/lyonbrown4d/orivis/internal/shared/logging"
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
			cfg, err := config.LoadFromFlags(cmd.Flags(), configFile)
			if err != nil {
				return err
			}

			logger, err := logging.New(cfg.Log.Level)
			if err != nil {
				return err
			}
			defer func() { _ = logging.Close(logger) }()

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			if err := serverapp.Run(ctx, cfg, logger); err != nil && !errors.Is(err, context.Canceled) {
				logger.Error("server exited", "error", err)
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&configFile, "config", "", "config file path")
	cmd.Flags().String("app-env", "", "runtime environment")
	cmd.Flags().String("http-addr", "", "HTTP listen address")
	cmd.Flags().String("log-level", "", "log level")
	cmd.Flags().String("db-driver", "", "database driver")
	cmd.Flags().String("db-dsn", "", "database DSN")
	cmd.Flags().String("auth-agent-token", "", "agent shared token")
	cmd.Flags().Bool("observability-prometheus-enabled", false, "enable Prometheus observability adapter")
	cmd.Flags().String("observability-prometheus-namespace", "", "Prometheus metric namespace")

	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
