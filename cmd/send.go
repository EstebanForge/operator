package cmd

import (
	"fmt"
	"strings"

	"github.com/EstebanForge/operator/pkg/tmux"
	"github.com/spf13/cobra"
)

var (
	sendNoEnter bool
	sendRaw     bool
)

var sendCmd = &cobra.Command{
	Use:   "send <name> <payload>",
	Short: "Inject keys into a target tmux session",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 2 {
			HandleError(fmt.Errorf("%w: session name and payload are required", tmux.ErrValidation))
			return
		}

		name := strings.TrimSpace(args[0])
		// Join all remaining words so an unquoted multi-word payload is not truncated.
		payload := strings.Join(args[1:], " ")
		enter := !sendNoEnter

		err := client.SendKeys(cmd.Context(), name, payload, enter, sendRaw)
		if err != nil {
			HandleError(err)
			return
		}

		if jsonFlag {
			resp := map[string]any{
				"status":  "ok",
				"action":  "send",
				"session": name,
				"details": map[string]any{
					"payload": payload,
					"enter":   enter,
					"raw":     sendRaw,
				},
			}
			if err := OutputJSON(resp); err != nil {
				HandleError(err)
			}
			return
		}

		fmt.Printf("Sent payload to session '%s'.\n", name)
	},
}

func init() {
	sendCmd.Flags().BoolVar(&sendNoEnter, "no-enter", false, "Do not send trailing Enter keystroke")
	sendCmd.Flags().BoolVar(&sendRaw, "raw", false, "Interpret payload as raw tmux keycodes instead of literal text")
	rootCmd.AddCommand(sendCmd)
}
