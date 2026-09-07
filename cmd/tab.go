package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/EstebanForge/operator/pkg/tmux"
	"github.com/spf13/cobra"
)

var (
	tabDir  string
	tabName string
)

var tabCmd = &cobra.Command{
	Use:   "tab <session>",
	Short: "Create a new tab (window) in an existing session",
	Run: func(cmd *cobra.Command, args []string) {
		var name string
		var hintDir string
		if len(args) > 0 {
			name = strings.TrimSpace(args[0])
		}

		if name == "" {
			if !isTerminal() {
				HandleError(fmt.Errorf("%w: session name required in non-interactive mode", tmux.ErrValidation))
				return
			}

			picked, err := pickSession(cmd.Context(), "Select Session for New Tab")
			if err != nil {
				if errors.Is(err, errAborted) {
					return
				}
				HandleError(err)
				return
			}
			name = picked
			// Prefer the picked session's pane path as the dir default; the
			// resolver validates it on disk.
			sessions, listErr := client.ListSessions(cmd.Context())
			if listErr == nil {
				for _, s := range sessions {
					if s.Name == name {
						hintDir = s.Path
						break
					}
				}
			}
		}

		dir := resolveTabDir(cmd.Context(), name, hintDir, tabDir)
		res, err := client.NewWindow(cmd.Context(), name, tabName, dir)
		if err != nil {
			HandleError(err)
			return
		}

		if jsonFlag {
			resp := map[string]any{
				"status":  "ok",
				"action":  "tab",
				"session": name,
				"details": map[string]any{
					"window":    res.Window,
					"index":     res.Index,
					"name":      res.Name,
					"directory": res.Directory,
				},
			}
			if err := OutputJSON(resp); err != nil {
				HandleError(err)
			}
			return
		}

		fmt.Printf("Tab '%s' created in '%s' (index %d, %s).\n", res.Name, name, res.Index, res.Directory)
	},
}

func init() {
	tabCmd.Flags().StringVarP(&tabDir, "dir", "d", "", "Working directory for the new tab")
	tabCmd.Flags().StringVarP(&tabName, "name", "n", "", "Name for the new tab (defaults to the shell name)")
	rootCmd.AddCommand(tabCmd)
}

// resolveTabDir picks the working directory for a new tab: explicit flag,
// then the picker's session pane path, then the session's current path from
// a fresh listing, then the operator cwd. Candidates are checked on disk;
// the first valid directory wins, so a deleted session directory can never
// fail a creation the user defaulted.
func resolveTabDir(ctx context.Context, session, hintDir, flagDir string) string {
	for _, cand := range []string{flagDir, hintDir} {
		if dirOK(cand) {
			return cand
		}
	}
	if sessions, err := client.ListSessions(ctx); err == nil {
		if idx := slices.IndexFunc(sessions, func(s tmux.Session) bool { return s.Name == session }); idx >= 0 && dirOK(sessions[idx].Path) {
			return sessions[idx].Path
		}
	}
	if wd, err := os.Getwd(); err == nil && dirOK(wd) {
		return wd
	}
	return ""
}

// dirOK reports whether path exists and is a directory.
func dirOK(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
