package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	agentapp "github.com/lyonbrown4d/orivis/internal/agent/app"
	"github.com/lyonbrown4d/orivis/internal/agent/config"
	"github.com/lyonbrown4d/orivis/internal/shared/logging"
	"github.com/spf13/cobra"
)

func main() {
	var configFile string

	cmd := &cobra.Command{
		Use:           "orivis-agent",
		Short:         "Run the Orivis agent",
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

			if err := agentapp.Run(ctx, cfg, logger); err != nil && !errors.Is(err, context.Canceled) {
				logger.Error("agent exited", "error", err)
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&configFile, "config", "", "config file path")
	cmd.Flags().String("server-url", "", "server base URL")
	cmd.Flags().String("agent-name", "", "agent name")
	cmd.Flags().String("agent-token", "", "agent token")
	cmd.Flags().String("agent-region", "", "agent region")
	cmd.Flags().String("runtime", "", "agent runtime")
	cmd.Flags().Duration("poll-interval", 0, "task polling interval")
	cmd.Flags().String("log-level", "", "log level")

	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
