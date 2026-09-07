package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/EstebanForge/operator/pkg/tmux"
	"github.com/charmbracelet/huh"
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
			sessions, err := client.ListSessions(cmd.Context())
			if err != nil {
				HandleError(err)
				return
			}
			if len(sessions) == 0 {
				HandleError(fmt.Errorf("%w: no active tmux sessions", tmux.ErrNotFound))
				return
			}

			options := make([]huh.Option[string], 0, len(sessions))
			for _, s := range sessions {
				label := fmt.Sprintf("%s (%d windows)", s.Name, s.Windows)
				options = append(options, huh.NewOption(label, s.Name))
			}

			err = runField(cmd.Context(), huh.NewSelect[string]().
				Title("Select Session to Join").
				Description(tuiMenuHint).
				Options(options...).
				Value(&name))
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					return
				}
				HandleError(err)
				return
			}
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
