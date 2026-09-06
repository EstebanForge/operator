package cmd

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Create 'opr' shorthand symlink beside operator binary",
	Run: func(_ *cobra.Command, _ []string) {
		execPath, err := os.Executable()
		if err != nil {
			HandleError(fmt.Errorf("unable to resolve executable path: %w", err))
		}

		realPath, _ := filepath.EvalSymlinks(execPath)
		realPath = cmp.Or(realPath, execPath)

		dir := filepath.Dir(realPath)
		binaryName := filepath.Base(realPath)
		symlinkPath := filepath.Join(dir, "opr")

		if _, err := os.Lstat(symlinkPath); err == nil {
			// Already exists; remove existing symlink if it points to this binary
			_ = os.Remove(symlinkPath) //nolint:errcheck // best-effort removal before re-link
		}

		err = os.Symlink(binaryName, symlinkPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Notice: could not create symlink at %s: %s\n", symlinkPath, err)
			fmt.Fprintln(os.Stderr, "Manual setup instruction:")
			fmt.Fprintf(os.Stderr, "  alias opr=\"%s\"\n", realPath)
			return
		}

		if jsonFlag {
			resp := map[string]any{
				"status":  "ok",
				"action":  "setup",
				"symlink": symlinkPath,
				"target":  realPath,
			}
			if err := OutputJSON(resp); err != nil {
				HandleError(err)
			}
			return
		}

		fmt.Printf("Symlink created: %s -> %s\n", symlinkPath, binaryName)
		fmt.Println("You can now invoke operator with 'opr'.")
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
