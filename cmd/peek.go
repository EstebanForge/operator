package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/EstebanForge/operator/pkg/tmux"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var peekLines int

var peekCmd = &cobra.Command{
	Use:   "peek [name]",
	Short: "Preview output from an active session pane",
	Run: func(cmd *cobra.Command, args []string) {
		var name string
		if len(args) > 0 {
			name = strings.TrimSpace(args[0])
		}

		if name == "" {
			if !isTerminal() {
				HandleError(fmt.Errorf("%w: session name required in non-interactive mode", tmux.ErrValidation))
			}

			sessions, err := client.ListSessions(cmd.Context())
			if err != nil {
				HandleError(err)
			}
			if len(sessions) == 0 {
				HandleError(fmt.Errorf("%w: no active tmux sessions", tmux.ErrNotFound))
			}

			options := make([]huh.Option[string], 0, len(sessions))
			for _, s := range sessions {
				label := fmt.Sprintf("%s (%d windows)", s.Name, s.Windows)
				options = append(options, huh.NewOption(label, s.Name))
			}

			err = runField(cmd.Context(), huh.NewSelect[string]().
				Title("Select Session to Peek").
				Options(options...).
				Value(&name))
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					return
				}
				HandleError(err)
			}
		}

		output, err := client.CapturePane(cmd.Context(), name, peekLines)
		if err != nil {
			HandleError(err)
		}

		if jsonFlag {
			resp := map[string]any{
				"session":        name,
				"lines_captured": peekLines,
				"output":         output,
			}
			if err := OutputJSON(resp); err != nil {
				HandleError(err)
			}
			return
		}

		fmt.Print(output)
	},
}

func init() {
	peekCmd.Flags().IntVarP(&peekLines, "lines", "l", 25, "Number of lines to capture")
	rootCmd.AddCommand(peekCmd)
}
