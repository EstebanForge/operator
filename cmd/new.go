package cmd

import (
	"cmp"
	"errors"
	"fmt"
	"strings"

	"github.com/EstebanForge/operator/pkg/tmux"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var (
	newDir      string
	newCmdStr   string
	newDetached bool
)

var newCmd = &cobra.Command{
	Use:   "new [name]",
	Short: "Create a new tmux session",
	Run: func(cmd *cobra.Command, args []string) {
		var name string
		if len(args) > 0 {
			name = strings.TrimSpace(args[0])
		}

		if name == "" {
			if !isTerminal() {
				name = tmux.GenerateUniqueSessionName(cmd.Context(), client)
			} else {
				autoName := tmux.GenerateUniqueSessionName(cmd.Context(), client)
				err := runField(cmd.Context(), huh.NewInput().
					Title("Session Name").
					Description("Press Enter to use auto-generated name: "+autoName).
					Placeholder(autoName).
					Value(&name).
					Validate(func(s string) error {
						trimmed := strings.TrimSpace(s)
						if trimmed != "" && tmux.SanitizeSessionName(trimmed) == "" {
							return fmt.Errorf("session name contains invalid characters")
						}
						return nil
					}))
				if err != nil {
					if errors.Is(err, huh.ErrUserAborted) {
						return
					}
					HandleError(err)
				}
				name = cmp.Or(strings.TrimSpace(name), autoName)
			}
		}

		sanitized := tmux.SanitizeSessionName(name)
		if sanitized == "" {
			HandleError(fmt.Errorf("%w: invalid session name", tmux.ErrValidation))
			return
		}

		if !newDetached && !isTerminal() {
			HandleError(tmux.ErrNotATTY)
			return
		}

		sess, err := client.NewSession(cmd.Context(), sanitized, newDir, newCmdStr, newDetached)
		if err != nil {
			if !newDetached && errors.Is(err, tmux.ErrSessionExited) {
				// The attached session closed normally after use: success, not an error.
				if jsonFlag {
					resp := map[string]any{
						"status":  "ok",
						"action":  "create",
						"session": sanitized,
						"details": map[string]any{
							"detached":  false,
							"directory": newDir,
							"exited":    true,
						},
					}
					if err := OutputJSON(resp); err != nil {
						HandleError(err)
					}
					return
				}
				printSessionExitHint(cmd.Context(), client, sanitized)
				return
			}
			HandleError(err)
			return
		}

		if jsonFlag {
			resp := map[string]any{
				"status":  "ok",
				"action":  "create",
				"session": sess.Name,
				"details": map[string]any{
					"detached":  newDetached,
					"directory": sess.Path,
				},
			}
			if err := OutputJSON(resp); err != nil {
				HandleError(err)
			}
			return
		}

		if newDetached {
			fmt.Printf("Session '%s' created (detached).\n", sess.Name)
		} else {
			// The user returned from the attached session: teach the loop.
			printSessionExitHint(cmd.Context(), client, sess.Name)
		}
	},
}

func init() {
	newCmd.Flags().StringVarP(&newDir, "dir", "d", "", "Working directory for session")
	newCmd.Flags().StringVarP(&newCmdStr, "cmd", "c", "", "Command to execute inside session")
	newCmd.Flags().BoolVar(&newDetached, "detached", false, "Create session without attaching")
	rootCmd.AddCommand(newCmd)
}
