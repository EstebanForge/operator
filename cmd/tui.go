package cmd

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/EstebanForge/operator/pkg/tmux"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/huh"
)

// runField runs an interactive huh.Field configured so that ESC and Ctrl+C abort.
var runField = runFieldWithEsc

func runFieldWithEsc(ctx context.Context, field huh.Field) error {
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(
		key.WithKeys("esc", "ctrl+c"),
		key.WithHelp("esc", "back/exit"),
	)
	form := huh.NewForm(huh.NewGroup(field)).
		WithShowHelp(false).
		WithKeyMap(km)
	if jsonFlag {
		form = form.WithOutput(os.Stderr)
	}
	return form.RunWithContext(ctx)
}

// tuiMenuHint is the static teaching line shown on interactive session screens.
const tuiMenuHint = "Inside tmux: Ctrl-b d detaches, exit closes."

// RunTUI launches the interactive menu loop.
func RunTUI(ctx context.Context, c tmux.Client) error {
	for {
		sessions, err := c.ListSessions(ctx)
		if err != nil {
			return err
		}

		sessionCount := len(sessions)
		title := fmt.Sprintf("operator | Active Sessions: %d", sessionCount)

		var action string
		var options []huh.Option[string]

		if sessionCount == 0 {
			options = []huh.Option[string]{
				huh.NewOption("Create New Session", "new"),
				huh.NewOption("Exit", "exit"),
			}
		} else {
			options = []huh.Option[string]{
				huh.NewOption("Attach to Session", "attach"),
				huh.NewOption("Create New Session", "new"),
				huh.NewOption("Peek Session Output", "peek"),
				huh.NewOption("Kill Session", "kill"),
				huh.NewOption("Exit", "exit"),
			}
		}

		err = runField(ctx, huh.NewSelect[string]().
			Title(title).
			Description(tuiMenuHint).
			Options(options...).
			Value(&action))
		if err != nil {
			return nil // Canceled or quit at menu root: quickly close/exit operator
		}

		switch action {
		case "exit":
			return nil

		case "new":
			autoName := tmux.GenerateUniqueSessionName(ctx, c)
			var name string
			attachNow := true
			step := 1

			for step > 0 && step <= 2 {
				switch step {
				case 1:
					err := runField(ctx, huh.NewInput().
						Title("Enter Session Name").
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
						// ESC at step 1: go back one level to root menu
						step = 0
						break
					}
					step = 2

				case 2:
					err := runField(ctx, huh.NewConfirm().
						Title("Attach to session immediately?").
						Value(&attachNow))
					if err != nil {
						// ESC at step 2: go back one level to step 1 (Enter Session Name)
						step = 1
						break
					}
					step = 3 // Proceed to creation
				}
			}

			if step == 0 {
				continue // back to root menu
			}

			name = cmp.Or(strings.TrimSpace(name), autoName)

			sanitized := tmux.SanitizeSessionName(name)
			_, err = c.NewSession(ctx, sanitized, "", "", !attachNow)
			if err != nil {
				if errors.Is(err, tmux.ErrSessionExited) {
					// User attached and exited tmux normally
					continue
				}
				fmt.Printf("Error creating session: %s\n", err)
				continue
			}
			if !attachNow {
				fmt.Printf("Session '%s' created.\n", sanitized)
			}

		case "attach":
			if len(sessions) == 0 {
				continue
			}
			var chosen string
			opts := make([]huh.Option[string], 0, len(sessions))
			for _, s := range sessions {
				label := fmt.Sprintf("%s (%d windows)", s.Name, s.Windows)
				opts = append(opts, huh.NewOption(label, s.Name))
			}
			err := runField(ctx, huh.NewSelect[string]().
				Title("Select Session to Attach").
				Options(opts...).
				Value(&chosen))
			if err != nil {
				// ESC at attach: go back one level to root menu
				continue
			}

			if err := c.Attach(ctx, chosen); err != nil {
				fmt.Printf("Attach failed: %s\n", err)
			}

		case "peek":
			if len(sessions) == 0 {
				continue
			}
			var chosen string
			opts := make([]huh.Option[string], 0, len(sessions))
			for _, s := range sessions {
				label := fmt.Sprintf("%s (%d windows)", s.Name, s.Windows)
				opts = append(opts, huh.NewOption(label, s.Name))
			}
			err := runField(ctx, huh.NewSelect[string]().
				Title("Select Session to Peek").
				Options(opts...).
				Value(&chosen))
			if err != nil {
				// ESC at peek: go back one level to root menu
				continue
			}

			out, err := c.CapturePane(ctx, chosen, 25)
			if err != nil {
				fmt.Printf("Peek failed: %s\n", err)
				continue
			}
			fmt.Println("--- Captured Output (Last 25 lines) ---")
			fmt.Println(out)
			fmt.Println("---------------------------------------")

		case "kill":
			if len(sessions) == 0 {
				continue
			}

			step := 1
			var chosen string
			var confirm bool

			for step > 0 && step <= 2 {
				switch step {
				case 1:
					opts := make([]huh.Option[string], 0, len(sessions)+1)
					opts = append(opts, huh.NewOption("[All Sessions]", "__all__"))
					for _, s := range sessions {
						label := fmt.Sprintf("%s (%d windows)", s.Name, s.Windows)
						opts = append(opts, huh.NewOption(label, s.Name))
					}
					err := runField(ctx, huh.NewSelect[string]().
						Title("Select Session to Kill").
						Options(opts...).
						Value(&chosen))
					if err != nil {
						// ESC at step 1: go back one level to root menu
						step = 0
						break
					}
					step = 2

				case 2:
					confirm = false
					target := fmt.Sprintf("session '%s'", chosen)
					if chosen == "__all__" {
						target = "ALL sessions"
					}
					err := runField(ctx, huh.NewConfirm().
						Title(fmt.Sprintf("Confirm killing %s?", target)).
						Value(&confirm))
					if err != nil {
						// ESC at step 2: go back one level to step 1 (Select Session to Kill)
						step = 1
						break
					}
					if !confirm {
						// Chose "No": go back one level to step 1 (Select Session to Kill)
						step = 1
						break
					}
					step = 3 // Confirmed, proceed to kill
				}
			}

			if step == 0 || !confirm {
				continue // back to root menu
			}

			var killErr error
			if chosen == "__all__" {
				killErr = c.Kill(ctx, "", true)
			} else {
				killErr = c.Kill(ctx, chosen, false)
			}

			target := fmt.Sprintf("session '%s'", chosen)
			if chosen == "__all__" {
				target = "ALL sessions"
			}
			if killErr != nil {
				fmt.Printf("Kill failed: %s\n", killErr)
			} else {
				fmt.Printf("Successfully killed %s.\n", target)
			}
		}
	}
}
