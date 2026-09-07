package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/EstebanForge/operator/pkg/tmux"
	"github.com/spf13/cobra"
)

var joinCmd = &cobra.Command{
	Use:   "join [name]",
	Short: "Attach to an existing tmux session",
	Run: func(cmd *cobra.Command, args []string) {
		var name string
		if len(args) > 0 {
			name = strings.TrimSpace(args[0])
		}

		if !isTerminal() {
			HandleError(fmt.Errorf("%w: attach requires an interactive terminal", tmux.ErrNotATTY))
			return
		}

		if name == "" {
			picked, err := pickSession(cmd.Context(), "Select Session to Join")
			if err != nil {
				if errors.Is(err, errAborted) {
					return
				}
				HandleError(err)
				return
			}
			name = picked
		}

		if err := client.Attach(cmd.Context(), name); err != nil {
			HandleError(err)
			return
		}

		// The tmux client returned: teach the detach/reattach/kill loop.
		printSessionExitHint(cmd.Context(), client, name)
	},
}

func init() {
	rootCmd.AddCommand(joinCmd)
}
