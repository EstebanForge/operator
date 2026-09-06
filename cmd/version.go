package cmd

import (
	"fmt"

	"github.com/EstebanForge/operator/internal/constants"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print operator version",
	Run: func(_ *cobra.Command, _ []string) {
		if jsonFlag {
			resp := map[string]any{
				"name":    constants.AppName,
				"version": constants.Version,
			}
			if err := OutputJSON(resp); err != nil {
				HandleError(err)
			}
			return
		}
		fmt.Printf("%s version %s\n", constants.AppName, constants.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
