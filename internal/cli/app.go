package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/lyonbrown4d/orivis/internal/shared/buildinfo"
	"github.com/spf13/cobra"
)

var ErrUsage = errors.New("usage error")

func Run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	root := NewCommand(stdout, stderr)
	root.SetArgs(args)
	if err := root.ExecuteContext(ctx); err != nil {
		message := err.Error()
		if strings.Contains(message, "unknown command") || strings.Contains(message, "unknown flag") {
			return fmt.Errorf("%w: %v", ErrUsage, err)
		}
		return err
	}

	return nil
}

func NewCommand(stdout io.Writer, stderr io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:           "orivis-cli",
		Short:         "Orivis operator CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, _ []string) error {
			info := buildinfo.Current()
			fmt.Fprintf(cmd.OutOrStdout(), "orivis %s (%s, %s)\n", info.Version, info.Commit, info.Date)
			return nil
		},
	})

	var healthAddr string
	var healthTimeout time.Duration
	healthCmd := &cobra.Command{
		Use:   "health",
		Short: "Check server health",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runHealth(cmd.Context(), healthAddr, healthTimeout, cmd.OutOrStdout())
		},
	}
	healthCmd.Flags().StringVar(&healthAddr, "addr", "http://127.0.0.1:8080", "server base URL")
	healthCmd.Flags().DurationVar(&healthTimeout, "timeout", 3*time.Second, "request timeout")
	root.AddCommand(healthCmd)

	return root
}

func runHealth(ctx context.Context, addr string, timeout time.Duration, stdout io.Writer) error {
	baseURL := strings.TrimRight(addr, "/")
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, baseURL+"/healthz", nil)
	if err != nil {
		return err
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	var payload map[string]string
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %s", response.Status)
	}

	fmt.Fprintf(stdout, "%s\n", payload["status"])
	return nil
}
