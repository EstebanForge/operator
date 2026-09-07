package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/EstebanForge/operator/pkg/tmux"
)

// detachHint is shown when the tmux client returns but the session is still
// alive: the user detached (Ctrl-b d) or exited a non-final window.
func detachHint(name string) string {
	return fmt.Sprintf("Detached. '%s' is still running.\nReattach: operator join %s\nStop it:  operator kill %s\n", name, name, name)
}

// closedHint is shown when the tmux client returns and the session is gone:
// the user exited its last shell.
func closedHint(name string) string {
	return fmt.Sprintf("Session '%s' closed.\n", name)
}

// printSessionExitHint prints a post-attach status line for humans.
// Skipped in JSON mode (machine contract) and inside tmux, where
// switch-client returns immediately while the user stays attached.
func printSessionExitHint(ctx context.Context, c tmux.Client, name string) {
	if jsonFlag || tmux.InsideTmux() {
		return
	}
	alive, err := c.HasSession(ctx, name)
	if err != nil {
		return // best-effort decoration; never mask the command result
	}
	if alive {
		fmt.Fprint(os.Stderr, detachHint(name))
		return
	}
	fmt.Fprint(os.Stderr, closedHint(name))
}
