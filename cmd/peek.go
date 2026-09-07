package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/EstebanForge/operator/pkg/tmux"
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
				return
			}

			picked, err := pickSession(cmd.Context(), "Select Session to Peek")
			if err != nil {
				if errors.Is(err, errAborted) {
					return
				}
				HandleError(err)
				return
			}
			name = picked
		}

		output, err := client.CapturePane(cmd.Context(), name, peekLines)
		if err != nil {
			HandleError(err)
			return
		}

		if jsonFlag {
			resp := map[string]any{
				"session":        name,
				"lines_captured": countLines(output),
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

// countLines reports the number of pane lines in a capture-pane -p payload.
// It may be lower than the requested -l value when the pane history is short.
func countLines(output string) int {
	trimmed := strings.TrimRight(output, "\n")
	if trimmed == "" {
		return 0
	}
	return strings.Count(trimmed, "\n") + 1
}

func init() {
	peekCmd.Flags().IntVarP(&peekLines, "lines", "l", 25, "Number of lines to capture")
	rootCmd.AddCommand(peekCmd)
}
