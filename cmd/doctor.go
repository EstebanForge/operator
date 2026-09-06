package cmd

import (
	"fmt"

	"github.com/EstebanForge/operator/pkg/tmux"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check tmux binary, version, and server status",
	Run: func(cmd *cobra.Command, _ []string) {
		res, err := client.Doctor(cmd.Context())
		if err != nil {
			HandleError(err)
		}

		if jsonFlag {
			if err := OutputJSON(res); err != nil {
				HandleError(err)
			}
			return
		}

		fmt.Println("operator doctor diagnostics:")
		if res.TmuxInstalled {
			fmt.Printf("  [✓] tmux installed: %s\n", res.TmuxPath)
		} else {
			fmt.Println("  [✗] tmux not found in PATH")
			info := tmux.DetectInstallInfo()
			fmt.Printf("\n  To install tmux for %s:\n    $ %s\n\n", info.Platform, info.Command)
		}

		if res.TmuxVersion != "" {
			fmt.Printf("  [✓] tmux version:   %s\n", res.TmuxVersion)
		} else {
			fmt.Println("  [✗] unable to determine tmux version")
		}

		if res.ServerRunning {
			fmt.Println("  [✓] server state:   running")
		} else {
			fmt.Println("  [-] server state:   idle (no active server)")
		}
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
