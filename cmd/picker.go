package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/EstebanForge/operator/pkg/tmux"
	"github.com/charmbracelet/huh"
)

// errAborted reports that the user backed out of a picker or prompt. It
// wraps the huh sentinel so callers do not depend on the TUI library.
var errAborted = huh.ErrUserAborted

// pickSession renders the shared interactive session picker used by the
// session-taking commands. Returns huh.ErrUserAborted when the user backs
// out, tmux.ErrNotFound when no sessions exist.
func pickSession(ctx context.Context, title string) (string, error) {
	sessions, err := client.ListSessions(ctx)
	if err != nil {
		return "", err
	}
	if len(sessions) == 0 {
		return "", fmt.Errorf("%w: no active tmux sessions", tmux.ErrNotFound)
	}

	options := make([]huh.Option[string], 0, len(sessions))
	for _, s := range sessions {
		label := fmt.Sprintf("%s (%d windows)", s.Name, s.Windows)
		options = append(options, huh.NewOption(label, s.Name))
	}

	var name string
	err = runField(ctx, huh.NewSelect[string]().
		Title(title).
		Description(tuiMenuHint).
		Options(options...).
		Value(&name))
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", errAborted
		}
		return "", err
	}
	return name, nil
}
