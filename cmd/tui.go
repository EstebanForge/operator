package cmd

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
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

// Root-level sentinel values for the session-first menu.
const (
	rootNew    = "__new__"
	rootServer = "__server__"
	rootExit   = "__exit__"
)

// RunTUI launches the interactive menu loop. The root level is the session
// list itself (session-first); picking a session opens its action submenu.
func RunTUI(ctx context.Context, c tmux.Client) error {
	for {
		sessions, err := c.ListSessions(ctx)
		if err != nil {
			return err
		}

		chosen, err := selectRoot(ctx, sessions)
		if err != nil {
			return nil // Esc at root closes operator
		}

		switch chosen {
		case rootExit:
			return nil
		case rootNew:
			tuiNewSession(ctx, c)
		case rootServer:
			tuiServerMenu(ctx, c)
		default:
			tuiSessionMenu(ctx, c, chosen)
		}
	}
}

// selectRoot renders the root menu: one entry per session, plus global
// commands. Sessions carry their window count in the label.
func selectRoot(ctx context.Context, sessions []tmux.Session) (string, error) {
	opts := make([]huh.Option[string], 0, len(sessions)+3)
	for _, s := range sessions {
		opts = append(opts, huh.NewOption(fmt.Sprintf("%s (%d windows)", s.Name, s.Windows), s.Name))
	}
	opts = append(opts,
		huh.NewOption("Create New Session", rootNew),
		huh.NewOption("Tmux Server Management", rootServer),
		huh.NewOption("Exit", rootExit),
	)

	title := fmt.Sprintf("operator | Active Sessions: %d", len(sessions))
	if len(sessions) == 0 {
		title = "operator | No Active Sessions"
	}

	var chosen string
	err := runField(ctx, huh.NewSelect[string]().
		Title(title).
		Description(tuiMenuHint).
		Options(opts...).
		Value(&chosen))
	return chosen, err
}

// confirmServerAction asks the user to approve a server-wide action. Esc
// or a declined prompt cancels.
func confirmServerAction(ctx context.Context, title string) bool {
	var confirm bool
	err := runField(ctx, huh.NewConfirm().Title(title).Value(&confirm))
	return err == nil && confirm
}

// tuiServerMenu shows server-level maintenance actions. Restart and kill
// end every session (and the programs running inside them), so both ask
// for confirmation and return to the root list afterward: the session
// state changed. Reload re-applies the tmux config without touching
// sessions; Status reports the tmux version and server state. Esc or Back
// returns to the root session list.
func tuiServerMenu(ctx context.Context, c tmux.Client) {
	for {
		var action string
		err := runField(ctx, huh.NewSelect[string]().
			Title("Tmux Server Management").
			Description("Server actions affect every session at once.").
			Options(
				huh.NewOption("Restart Tmux Server", "restart"),
				huh.NewOption("Kill Tmux Server", "kill"),
				huh.NewOption("Reload Config", "reload"),
				huh.NewOption("Server Status", "status"),
				huh.NewOption("Back", "back"),
			).
			Value(&action))
		if err != nil {
			return // Esc = Back
		}

		switch action {
		case "back":
			return

		case "restart":
			if !confirmServerAction(ctx, "Restart the tmux server? Every session and its programs end. A fresh 'main' session starts.") {
				continue
			}
			sess, err := c.RestartServer(ctx)
			if err != nil {
				fmt.Printf("Restart failed: %s\n", err)
				continue
			}
			fmt.Printf("Server restarted. Fresh session '%s' created.\n", sess.Name)
			return // session list changed: refresh the root list

		case "kill":
			if !confirmServerAction(ctx, "Kill the tmux server? Every session and its programs end.") {
				continue
			}
			if err := c.KillServer(ctx); err != nil {
				fmt.Printf("Kill failed: %s\n", err)
				continue
			}
			fmt.Println("Tmux server killed.")
			return // session list changed: refresh the root list

		case "reload":
			if err := c.ReloadConfig(ctx); err != nil {
				fmt.Printf("Reload failed: %s\n", err)
				continue
			}
			fmt.Println("Tmux config reloaded.")

		case "status":
			doc, err := c.Doctor(ctx)
			if err != nil {
				fmt.Printf("Status failed: %s\n", err)
				continue
			}
			if !doc.TmuxInstalled {
				fmt.Println("tmux is not installed.")
				continue
			}
			fmt.Printf("tmux: %s (%s)\nServer running: %v\n", doc.TmuxVersion, doc.TmuxPath, doc.ServerRunning)
		}
	}
}

// tuiSessionMenu shows the action submenu for one session. Session state is
// refreshed on every render: the session may die (attach exit, external
// kill) or grow tabs between screens. Returns to the root list when the
// session disappears, on Back, or on Esc.
func tuiSessionMenu(ctx context.Context, c tmux.Client, name string) {
	for {
		sessions, err := c.ListSessions(ctx)
		if err != nil {
			return // the root loop surfaces the failure
		}
		idx := slices.IndexFunc(sessions, func(cur tmux.Session) bool { return cur.Name == name })
		if idx < 0 {
			return // session gone: refresh the root list
		}
		s := sessions[idx]

		var action string
		err = runField(ctx, huh.NewSelect[string]().
			Title(fmt.Sprintf("%s (%d windows)", s.Name, s.Windows)).
			Options(
				huh.NewOption("Attach to Session", "attach"),
				huh.NewOption("Create Tab in Session", "tab"),
				huh.NewOption("Peek Session Output", "peek"),
				huh.NewOption("Kill Session", "kill"),
				huh.NewOption("Back", "back"),
			).
			Value(&action))
		if err != nil {
			return // Esc = Back
		}

		switch action {
		case "back":
			return

		case "attach":
			if err := c.Attach(ctx, s.Name); err != nil {
				fmt.Printf("Attach failed: %s\n", err)
				continue
			}
			// The tmux client returned: teach the detach/reattach/kill loop.
			printSessionExitHint(ctx, c, s.Name)

		case "tab":
			tuiCreateTab(ctx, c, s)

		case "peek":
			out, err := c.CapturePane(ctx, s.Name, 25)
			if err != nil {
				fmt.Printf("Peek failed: %s\n", err)
				continue
			}
			fmt.Println("--- Captured Output (Last 25 lines) ---")
			fmt.Println(out)
			fmt.Println("---------------------------------------")

		case "kill":
			var confirm bool
			err := runField(ctx, huh.NewConfirm().
				Title(fmt.Sprintf("Confirm killing session '%s'?", s.Name)).
				Value(&confirm))
			if err != nil {
				continue // Esc = Back
			}
			if !confirm {
				continue
			}
			if err := c.Kill(ctx, s.Name, false); err != nil {
				fmt.Printf("Kill failed: %s\n", err)
				continue
			}
			fmt.Printf("Successfully killed session '%s'.\n", s.Name)
			return // session gone: refresh the root list
		}
	}
}

// tuiCreateTab prompts for the working directory (default: the session's
// active pane path) and an optional name, then creates the tab. Esc at the
// directory prompt returns to the session menu; Esc at the name prompt
// returns to the directory prompt.
func tuiCreateTab(ctx context.Context, c tmux.Client, s tmux.Session) {
	dir := s.Path
	var name string
	step := 1

	for step > 0 && step <= 2 {
		switch step {
		case 1:
			err := runField(ctx, huh.NewInput().
				Title(fmt.Sprintf("Working directory for new tab in '%s'", s.Name)).
				Description("Enter accepts the default. Esc goes back.").
				Value(&dir))
			if err != nil {
				step = 0
				break
			}
			step = 2

		case 2:
			err := runField(ctx, huh.NewInput().
				Title("Tab name (optional)").
				Description("Enter lets tmux name it after the shell. Esc goes back.").
				Value(&name))
			if err != nil {
				step = 1
				break
			}
			step = 3
		}
	}
	if step == 0 {
		return
	}

	res, err := c.NewWindow(ctx, s.Name, strings.TrimSpace(name), strings.TrimSpace(dir))
	if err != nil {
		fmt.Printf("Tab creation failed: %s\n", err)
		return
	}
	fmt.Printf("Tab '%s' created in '%s' (index %d, %s).\n", res.Name, s.Name, res.Index, res.Directory)
}

// tuiNewSession runs the guided new-session flow: name (auto-generated
// default) then attach confirmation. Esc steps back one level at a time.
func tuiNewSession(ctx context.Context, c tmux.Client) {
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
		return // back to root menu
	}

	name = cmp.Or(strings.TrimSpace(name), autoName)

	sanitized := tmux.SanitizeSessionName(name)
	_, err := c.NewSession(ctx, sanitized, "", "", !attachNow)
	if err != nil {
		if errors.Is(err, tmux.ErrSessionExited) {
			// User attached and exited tmux normally.
			printSessionExitHint(ctx, c, sanitized)
			return
		}
		fmt.Printf("Error creating session: %s\n", err)
		return
	}
	if !attachNow {
		fmt.Printf("Session '%s' created.\n", sanitized)
	}
}
