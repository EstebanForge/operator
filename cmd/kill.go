package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/EstebanForge/operator/pkg/tmux"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var (
	killAll   bool
	killForce bool
)

var killCmd = &cobra.Command{
	Use:   "kill [name]",
	Short: "Kill a tmux session or all sessions",
	Run: func(cmd *cobra.Command, args []string) {
		if killAll && len(args) > 0 {
			HandleError(fmt.Errorf("%w: cannot specify session name with --all", tmux.ErrValidation))
			return
		}

		var name string
		if len(args) > 0 {
			name = strings.TrimSpace(args[0])
		}

		if !killAll && name == "" {
			if !isTerminal() {
				HandleError(fmt.Errorf("%w: session name or --all required in non-interactive mode", tmux.ErrValidation))
				return
			}

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
				Title("Select Session to Kill").
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

		// Prompt confirmation in interactive mode unless --force is given
		if isTerminal() && !killForce {
			targetDesc := fmt.Sprintf("session '%s'", name)
			if killAll {
				targetDesc = "ALL tmux sessions and server"
			}

			var confirm bool
			err := runField(cmd.Context(), huh.NewConfirm().
				Title(fmt.Sprintf("Are you sure you want to kill %s?", targetDesc)).
				Value(&confirm))
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					return
				}
				HandleError(err)
				return
			}
			if !confirm {
				if jsonFlag {
					resp := map[string]any{
						"status": "canceled",
						"action": "kill",
					}
					if err := OutputJSON(resp); err != nil {
						HandleError(err)
					}
					return
				}
				fmt.Println("Canceled.")
				return
			}
		}

		err := client.Kill(cmd.Context(), name, killAll)
		if err != nil {
			HandleError(err)
			return
		}

		targetName := name
		if killAll {
			targetName = "all"
		}

		if jsonFlag {
			resp := map[string]any{
				"status":  "ok",
				"action":  "kill",
				"session": targetName,
				"details": map[string]any{
					"all": killAll,
				},
			}
			if err := OutputJSON(resp); err != nil {
				HandleError(err)
			}
			return
		}

		if killAll {
			fmt.Println("Killed all tmux sessions.")
		} else {
			fmt.Printf("Killed session '%s'.\n", name)
		}
	},
}

func init() {
	killCmd.Flags().BoolVarP(&killAll, "all", "a", false, "Kill all tmux sessions")
	killCmd.Flags().BoolVarP(&killForce, "force", "f", false, "Skip interactive confirmation")
	rootCmd.AddCommand(killCmd)
}
