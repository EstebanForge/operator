// Package cmd contains the CLI commands and TUI loop for operator.
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/EstebanForge/operator/internal/constants"
	"github.com/EstebanForge/operator/pkg/tmux"
	"github.com/spf13/cobra"
)

var (
	jsonFlag bool
	client   tmux.Client = tmux.NewOSClient()

	isTerminal = tmux.IsTerminal

	rootCmd = &cobra.Command{
		Use:     "operator",
		Short:   "Zero-cruft session multiplexer and orchestrator for tmux",
		Version: constants.Version,
		Run: func(cmd *cobra.Command, _ []string) {
			if !isTerminal() {
				cmd.SetOut(os.Stderr)
				cmd.SetErr(os.Stderr)
				_ = cmd.Usage() //nolint:errcheck // best-effort usage print
				osExit(ExitValidation)
				return
			}

			if err := RunTUI(cmd.Context(), client); err != nil {
				HandleError(err)
			}
		},
	}
)

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Format command output as JSON")
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
}

// Execute executes the root command.
func Execute() {
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		if ResolveExitCode(err) == ExitDaemon {
			err = fmt.Errorf("%w: %w", tmux.ErrValidation, err)
		}
		HandleError(err)
	}
}

// SetClient overrides the client instance (useful for testing).
func SetClient(c tmux.Client) {
	client = c
}
