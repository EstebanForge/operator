package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List active tmux sessions",
	Run: func(cmd *cobra.Command, _ []string) {
		sessions, err := client.ListSessions(cmd.Context())
		if err != nil {
			HandleError(err)
		}

		if jsonFlag {
			if err := OutputJSON(sessions); err != nil {
				HandleError(err)
			}
			return
		}

		if len(sessions) == 0 {
			fmt.Println("No active tmux sessions.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAME\tWINDOWS\tCREATED\tATTACHED\tPATH") //nolint:errcheck
		for _, s := range sessions {
			attached := "no"
			if s.IsAttached {
				attached = "yes"
			}
			_, _ = fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\n", s.Name, s.Windows, s.CreatedAt, attached, s.Path) //nolint:errcheck
		}
		_ = w.Flush() //nolint:errcheck
	},
}

func init() {
	rootCmd.AddCommand(lsCmd)
}
