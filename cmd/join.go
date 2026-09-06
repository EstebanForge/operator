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
		}

		if name == "" {
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
				Title("Select Session to Join").
				Options(options...).
				Value(&name))
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					return
				}
				HandleError(err)
			}
		}

		if err := client.Attach(cmd.Context(), name); err != nil {
			HandleError(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(joinCmd)
}
