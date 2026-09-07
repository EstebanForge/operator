package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// errSymlinkConflict reports an existing 'opr' that operator did not create.
var errSymlinkConflict = errors.New("refusing to overwrite non-operator symlink")

// osExecutable is overridden in tests.
var osExecutable = os.Executable

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Create 'opr' shorthand symlink beside operator binary",
	Run: func(_ *cobra.Command, _ []string) {
		execPath, err := osExecutable()
		if err != nil {
			HandleError(fmt.Errorf("unable to resolve executable path: %w", err))
			return
		}

		realPath, evalErr := filepath.EvalSymlinks(execPath)
		if evalErr != nil || realPath == "" {
			realPath = execPath
		}

		dir := filepath.Dir(realPath)
		binaryName := filepath.Base(realPath)
		symlinkPath := filepath.Join(dir, "opr")

		exists, owned := oprOwnership(symlinkPath, binaryName, realPath)
		if exists && !owned {
			HandleError(fmt.Errorf("%w: %s", errSymlinkConflict, symlinkPath))
			return
		}
		if exists {
			_ = os.Remove(symlinkPath) //nolint:errcheck // refresh managed link before re-link
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

// oprOwnership reports whether an 'opr' entry exists at path and whether it
// points at this operator binary. Owned links are refreshed on setup; foreign
// entries are never touched.
func oprOwnership(path, binaryName, realPath string) (exists, owned bool) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, false
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return true, false
	}
	target, err := os.Readlink(path)
	if err != nil {
		return true, false
	}
	return true, target == binaryName || target == realPath
}
