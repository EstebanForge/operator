// Package cmd contains the CLI commands and TUI loop for operator.
package cmd

import (
	"context"
	"os"

	"github.com/EstebanForge/operator/internal/constants"
	"github.com/EstebanForge/operator/pkg/tmux"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var (
	jsonFlag bool
	client   tmux.Client = tmux.NewOSClient()

	rootCmd = &cobra.Command{
		Use:     "operator",
		Short:   "Zero-cruft session multiplexer and orchestrator for tmux",
		Version: constants.Version,
		Run: func(cmd *cobra.Command, _ []string) {
			if !isTerminal() {
				cmd.SetOut(os.Stderr)
				cmd.SetErr(os.Stderr)
				_ = cmd.Usage() //nolint:errcheck // best-effort usage print
				os.Exit(ExitValidation)
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

func isTerminal() bool {
	return (isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())) &&
		(isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()))
}

// Execute executes the root command.
func Execute() {
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		HandleError(err)
	}
}

// SetClient overrides the client instance (useful for testing).
func SetClient(c tmux.Client) {
	client = c
}
