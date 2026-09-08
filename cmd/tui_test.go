package cmd

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/EstebanForge/operator/pkg/tmux"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

func TestEscapeKeybindingConfig(t *testing.T) {
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(
		key.WithKeys("esc", "ctrl+c"),
		key.WithHelp("esc", "back/exit"),
	)

	escMsg := tea.KeyMsg{Type: tea.KeyEscape}
	if !key.Matches(escMsg, km.Quit) {
		t.Fatalf("expected ESC to match Quit keybinding")
	}

	ctrlCMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	if !key.Matches(ctrlCMsg, km.Quit) {
		t.Fatalf("expected Ctrl+C to match Quit keybinding")
	}
}

func TestFormAbortsOnEscapeKey(t *testing.T) {
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(
		key.WithKeys("esc", "ctrl+c"),
		key.WithHelp("esc", "back/exit"),
	)

	var chosen string
	sel := huh.NewSelect[string]().
		Title("Select Option").
		Options(huh.NewOption("Option 1", "opt1"), huh.NewOption("Option 2", "opt2")).
		Value(&chosen)

	form := huh.NewForm(huh.NewGroup(sel)).
		WithShowHelp(false).
		WithKeyMap(km)
	form.CancelCmd = tea.Interrupt

	m, cmd := form.Update(tea.KeyMsg{Type: tea.KeyEscape})
	f, ok := m.(*huh.Form)
	if !ok {
		t.Fatalf("expected *huh.Form from Update")
	}
	if f.State != huh.StateAborted {
		t.Fatalf("expected form State to be StateAborted (%d), got %d", huh.StateAborted, f.State)
	}
	if cmd == nil {
		t.Fatalf("expected non-nil CancelCmd on ESC abort")
	}
}

// captureTUIStdout redirects os.Stdout while RunTUI prints human feedback
// (peek blocks, tab confirmations, kill results). The returned func closes
// the pipe and returns everything printed so far.
func captureTUIStdout(t *testing.T) func() string {
	t.Helper()
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })
	return func() string {
		_ = w.Close()
		os.Stdout = old
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		_ = r.Close()
		return buf.String()
	}
}

// pressEnter selects the currently highlighted option of a select field.
func pressEnter(f huh.Field) {
	sel, ok := f.(*huh.Select[string])
	if !ok {
		return
	}
	sel.Init()
	sel.Update(tea.KeyMsg{Type: tea.KeyEnter})
}

// pressEnterOnInput submits the current value of an input field.
func pressEnterOnInput(f huh.Field) {
	input, ok := f.(*huh.Input)
	if !ok {
		return
	}
	input.Init()
	input.Update(tea.KeyMsg{Type: tea.KeyEnter})
}

func TestRunTUI_RootEscapeExits(t *testing.T) {
	origRunField := runField
	defer func() { runField = origRunField }()

	callCount := 0
	runField = func(_ context.Context, _ huh.Field) error {
		callCount++
		// Simulate ESC pressed at root menu
		return huh.ErrUserAborted
	}

	mock := &mockTmuxClient{
		sessions: []tmux.Session{
			{Name: "sess1", Windows: 1},
		},
	}

	err := RunTUI(t.Context(), mock)
	if err != nil {
		t.Fatalf("expected RunTUI to return nil on root ESC, got: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected exactly 1 call to runField, got %d", callCount)
	}
}

func TestRunTUI_SessionFirst_AttachFlow(t *testing.T) {
	origRunField := runField
	defer func() { runField = origRunField }()
	t.Setenv("TMUX", "/tmp/tmux-0/default,1,0") // silence the post-attach hint on stderr

	step := 0
	runField = func(_ context.Context, f huh.Field) error {
		f.WithKeyMap(huh.NewDefaultKeyMap())
		step++
		switch step {
		case 1:
			// Root: the first option is the session itself; select it.
			pressEnter(f)
			return nil
		case 2:
			// Session submenu: "Attach to Session" is first; select it.
			pressEnter(f)
			return nil
		case 3:
			// Back in the submenu after attach: ESC = Back.
			return huh.ErrUserAborted
		case 4:
			// Root menu again: ESC exits.
			return huh.ErrUserAborted
		default:
			t.Fatalf("unexpected step %d", step)
			return nil
		}
	}

	mock := &mockTmuxClient{
		sessions: []tmux.Session{{Name: "sess1", Windows: 1}},
	}

	err := RunTUI(t.Context(), mock)
	if err != nil {
		t.Fatalf("expected RunTUI to return nil, got: %v", err)
	}
	if step != 4 {
		t.Fatalf("expected 4 steps (root -> submenu attach -> submenu ESC -> root ESC), got %d", step)
	}
}

func TestRunTUI_SessionFirst_PeekFlow(t *testing.T) {
	origRunField := runField
	defer func() { runField = origRunField }()

	capture := captureTUIStdout(t)

	step := 0
	runField = func(_ context.Context, f huh.Field) error {
		f.WithKeyMap(huh.NewDefaultKeyMap())
		step++
		switch step {
		case 1:
			// Root: select the session.
			pressEnter(f)
			return nil
		case 2:
			// Session submenu: navigate to "Peek Session Output"
			// (attach -> tab -> peek = two downs).
			sel := f.(*huh.Select[string])
			sel.Init()
			sel.Update(tea.KeyMsg{Type: tea.KeyDown})
			sel.Update(tea.KeyMsg{Type: tea.KeyDown})
			sel.Update(tea.KeyMsg{Type: tea.KeyEnter})
			return nil
		case 3:
			// Back in the submenu after peek: ESC = Back.
			return huh.ErrUserAborted
		case 4:
			// Root menu: ESC exits.
			return huh.ErrUserAborted
		default:
			t.Fatalf("unexpected step %d", step)
			return nil
		}
	}

	mock := &mockTmuxClient{
		sessions: []tmux.Session{{Name: "sess1", Windows: 1}},
	}

	err := RunTUI(t.Context(), mock)
	if err != nil {
		t.Fatalf("expected RunTUI to return nil, got: %v", err)
	}
	if step != 4 {
		t.Fatalf("expected 4 steps, got %d", step)
	}
	if out := capture(); !bytes.Contains([]byte(out), []byte("Captured Output")) {
		t.Errorf("expected peek output block, got: %q", out)
	}
}

func TestRunTUI_SessionFirst_TabFlow(t *testing.T) {
	origRunField := runField
	defer func() { runField = origRunField }()

	capture := captureTUIStdout(t)

	step := 0
	runField = func(_ context.Context, f huh.Field) error {
		f.WithKeyMap(huh.NewDefaultKeyMap())
		step++
		switch step {
		case 1:
			// Root: select the session.
			pressEnter(f)
			return nil
		case 2:
			// Session submenu: "Create Tab in Session" (one down from attach).
			sel := f.(*huh.Select[string])
			sel.Init()
			sel.Update(tea.KeyMsg{Type: tea.KeyDown})
			sel.Update(tea.KeyMsg{Type: tea.KeyEnter})
			return nil
		case 3:
			// Tab flow step 1: working directory (prefilled) -> Enter.
			pressEnterOnInput(f)
			return nil
		case 4:
			// Tab flow step 2: optional name -> Enter.
			pressEnterOnInput(f)
			return nil
		case 5:
			// Back in the submenu: ESC = Back.
			return huh.ErrUserAborted
		case 6:
			// Root: ESC exits.
			return huh.ErrUserAborted
		default:
			t.Fatalf("unexpected step %d", step)
			return nil
		}
	}

	mock := &mockTmuxClient{
		sessions: []tmux.Session{{Name: "sess1", Windows: 1, Path: "/srv/sess1"}},
	}

	err := RunTUI(t.Context(), mock)
	if err != nil {
		t.Fatalf("expected RunTUI to return nil, got: %v", err)
	}
	if step != 6 {
		t.Fatalf("expected 6 steps, got %d", step)
	}
	if mock.lastWindow.Directory != "/srv/sess1" {
		t.Errorf("expected tab dir to default to the session path, got %q", mock.lastWindow.Directory)
	}
	if mock.lastWindow.Name != "bash" {
		t.Errorf("expected tmux default window name, got %q", mock.lastWindow.Name)
	}
	if out := capture(); !bytes.Contains([]byte(out), []byte("created in 'sess1'")) {
		t.Errorf("expected tab confirmation, got: %q", out)
	}
}

func TestRunTUI_NewSession_EscLevels(t *testing.T) {
	origRunField := runField
	defer func() { runField = origRunField }()

	// Test: Root -> New -> Step 1 (Session Name) -> ESC -> Root -> ESC
	t.Run("ESC at step 1 goes back to root", func(t *testing.T) {
		step := 0
		runField = func(_ context.Context, f huh.Field) error {
			step++
			switch step {
			case 1:
				// Root menu (0 sessions): first option is "Create New Session".
				pressEnter(f)
				return nil
			case 2:
				// New flow step 1: session name input -> press ESC.
				return huh.ErrUserAborted
			case 3:
				// Returned to root -> press ESC to exit.
				return huh.ErrUserAborted
			default:
				t.Fatalf("unexpected step %d", step)
				return nil
			}
		}

		mock := &mockTmuxClient{}
		err := RunTUI(t.Context(), mock)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if step != 3 {
			t.Fatalf("expected 3 steps, got %d", step)
		}
	})

	// Test: Root -> New -> Step 1 (enter) -> Step 2 (confirm) -> ESC -> Step 1 -> ESC -> Root -> ESC
	t.Run("ESC at step 2 goes back to step 1", func(t *testing.T) {
		step := 0
		runField = func(_ context.Context, f huh.Field) error {
			step++
			switch step {
			case 1:
				// Root: select "Create New Session".
				pressEnter(f)
				return nil
			case 2:
				// New flow step 1: session name input -> press Enter.
				pressEnterOnInput(f)
				return nil
			case 3:
				// New flow step 2: confirm attach -> press ESC (back to step 1).
				return huh.ErrUserAborted
			case 4:
				// New flow step 1 again: press ESC (back to root).
				return huh.ErrUserAborted
			case 5:
				// Root: press ESC to exit.
				return huh.ErrUserAborted
			default:
				t.Fatalf("unexpected step %d", step)
				return nil
			}
		}

		mock := &mockTmuxClient{}
		err := RunTUI(t.Context(), mock)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if step != 5 {
			t.Fatalf("expected 5 steps, got %d", step)
		}
	})
}

func TestRunTUI_KillSession_EscLevels(t *testing.T) {
	origRunField := runField
	defer func() { runField = origRunField }()

	selectKillInSubmenu := func(f huh.Field) {
		sel := f.(*huh.Select[string])
		sel.Init()
		// attach -> tab -> peek -> kill = three downs.
		for range 3 {
			sel.Update(tea.KeyMsg{Type: tea.KeyDown})
		}
		sel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	}

	// Test: Root -> session -> submenu Kill -> confirm ESC -> submenu ESC -> root ESC
	t.Run("ESC at confirm goes back to submenu", func(t *testing.T) {
		step := 0
		runField = func(_ context.Context, f huh.Field) error {
			f.WithKeyMap(huh.NewDefaultKeyMap())
			step++
			switch step {
			case 1:
				// Root: select the session.
				pressEnter(f)
				return nil
			case 2:
				// Session submenu: select "Kill Session".
				selectKillInSubmenu(f)
				return nil
			case 3:
				// Kill confirm: press ESC (back to submenu).
				return huh.ErrUserAborted
			case 4:
				// Session submenu: press ESC (back to root).
				return huh.ErrUserAborted
			case 5:
				// Root: press ESC to exit.
				return huh.ErrUserAborted
			default:
				t.Fatalf("unexpected step %d", step)
				return nil
			}
		}

		mock := &mockTmuxClient{sessions: []tmux.Session{{Name: "worker1", Windows: 1}}}
		err := RunTUI(t.Context(), mock)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if step != 5 {
			t.Fatalf("expected 5 steps, got %d", step)
		}
	})

	// Test: rejecting the confirm ("No") stays in the submenu.
	t.Run("Rejecting confirm stays in submenu", func(t *testing.T) {
		step := 0
		runField = func(_ context.Context, f huh.Field) error {
			f.WithKeyMap(huh.NewDefaultKeyMap())
			step++
			switch step {
			case 1:
				// Root: select the session.
				pressEnter(f)
				return nil
			case 2:
				// Session submenu: select "Kill Session".
				selectKillInSubmenu(f)
				return nil
			case 3:
				// Kill confirm: choose "No".
				confirm := f.(*huh.Confirm)
				confirm.Init()
				confirm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
				return nil
			case 4:
				// Session submenu: press ESC (back to root).
				return huh.ErrUserAborted
			case 5:
				// Root: press ESC to exit.
				return huh.ErrUserAborted
			default:
				t.Fatalf("unexpected step %d", step)
				return nil
			}
		}

		mock := &mockTmuxClient{sessions: []tmux.Session{{Name: "worker1", Windows: 1}}}
		err := RunTUI(t.Context(), mock)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if step != 5 {
			t.Fatalf("expected 5 steps, got %d", step)
		}
	})
}

// selectServerInRoot navigates the root select to "Tmux Server Management".
// It sits one slot above Exit: sessions..., Create New Session, Server, Exit.
func selectServerInRoot(f huh.Field, sessionCount int) {
	sel := f.(*huh.Select[string])
	sel.Init()
	for range sessionCount + 1 {
		sel.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	sel.Update(tea.KeyMsg{Type: tea.KeyEnter})
}

// selectServerAction navigates the server submenu to the nth option.
func selectServerAction(f huh.Field, index int) {
	sel := f.(*huh.Select[string])
	sel.Init()
	for range index {
		sel.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	sel.Update(tea.KeyMsg{Type: tea.KeyEnter})
}

func TestRunTUI_ServerMenu_EscBack(t *testing.T) {
	origRunField := runField
	defer func() { runField = origRunField }()

	step := 0
	runField = func(_ context.Context, f huh.Field) error {
		f.WithKeyMap(huh.NewDefaultKeyMap())
		step++
		switch step {
		case 1:
			// Root: select "Tmux Server Management".
			selectServerInRoot(f, 1)
			return nil
		case 2:
			// Server submenu: ESC = Back to root.
			return huh.ErrUserAborted
		case 3:
			// Root: ESC exits.
			return huh.ErrUserAborted
		default:
			t.Fatalf("unexpected step %d", step)
			return nil
		}
	}

	mock := &mockTmuxClient{sessions: []tmux.Session{{Name: "worker1", Windows: 1}}}
	if err := RunTUI(t.Context(), mock); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if step != 3 {
		t.Fatalf("expected 3 steps, got %d", step)
	}
}

func TestRunTUI_ServerMenu_KillDeclineStays(t *testing.T) {
	origRunField := runField
	defer func() { runField = origRunField }()

	step := 0
	runField = func(_ context.Context, f huh.Field) error {
		f.WithKeyMap(huh.NewDefaultKeyMap())
		step++
		switch step {
		case 1:
			selectServerInRoot(f, 1)
			return nil
		case 2:
			// Server submenu: select "Kill Tmux Server".
			selectServerAction(f, 1)
			return nil
		case 3:
			// Kill confirm: choose "No".
			confirm := f.(*huh.Confirm)
			confirm.Init()
			confirm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
			return nil
		case 4:
			// Server submenu: ESC = Back to root.
			return huh.ErrUserAborted
		case 5:
			// Root: ESC exits.
			return huh.ErrUserAborted
		default:
			t.Fatalf("unexpected step %d", step)
			return nil
		}
	}

	mock := &mockTmuxClient{sessions: []tmux.Session{{Name: "worker1", Windows: 1}}}
	if err := RunTUI(t.Context(), mock); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if step != 5 {
		t.Fatalf("expected 5 steps, got %d", step)
	}
	if mock.killedServer {
		t.Fatal("expected server to survive a declined kill")
	}
}

func TestRunTUI_ServerMenu_KillConfirmed(t *testing.T) {
	origRunField := runField
	defer func() { runField = origRunField }()

	capture := captureTUIStdout(t)

	step := 0
	runField = func(_ context.Context, f huh.Field) error {
		f.WithKeyMap(huh.NewDefaultKeyMap())
		step++
		switch step {
		case 1:
			selectServerInRoot(f, 1)
			return nil
		case 2:
			selectServerAction(f, 1) // "Kill Tmux Server"
			return nil
		case 3:
			// Kill confirm: choose "Yes".
			confirm := f.(*huh.Confirm)
			confirm.Init()
			confirm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
			return nil
		case 4:
			// Back at the refreshed root: ESC exits.
			return huh.ErrUserAborted
		default:
			t.Fatalf("unexpected step %d", step)
			return nil
		}
	}

	mock := &mockTmuxClient{sessions: []tmux.Session{{Name: "worker1", Windows: 1}}}
	if err := RunTUI(t.Context(), mock); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if step != 4 {
		t.Fatalf("expected 4 steps, got %d", step)
	}
	if !mock.killedServer {
		t.Fatal("expected KillServer to run after confirmation")
	}
	if out := capture(); !bytes.Contains([]byte(out), []byte("Tmux server killed")) {
		t.Errorf("expected kill confirmation output, got: %q", out)
	}
}

func TestRunTUI_ServerMenu_RestartConfirmed(t *testing.T) {
	origRunField := runField
	defer func() { runField = origRunField }()

	capture := captureTUIStdout(t)

	step := 0
	runField = func(_ context.Context, f huh.Field) error {
		f.WithKeyMap(huh.NewDefaultKeyMap())
		step++
		switch step {
		case 1:
			selectServerInRoot(f, 1)
			return nil
		case 2:
			// Server submenu: "Restart Tmux Server" is first.
			selectServerAction(f, 0)
			return nil
		case 3:
			// Restart confirm: choose "Yes".
			confirm := f.(*huh.Confirm)
			confirm.Init()
			confirm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
			return nil
		case 4:
			// Back at the refreshed root: ESC exits.
			return huh.ErrUserAborted
		default:
			t.Fatalf("unexpected step %d", step)
			return nil
		}
	}

	mock := &mockTmuxClient{sessions: []tmux.Session{{Name: "worker1", Windows: 1}}}
	if err := RunTUI(t.Context(), mock); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if step != 4 {
		t.Fatalf("expected 4 steps, got %d", step)
	}
	if !mock.restartedServer {
		t.Fatal("expected RestartServer to run after confirmation")
	}
	if out := capture(); !bytes.Contains([]byte(out), []byte("Server restarted")) {
		t.Errorf("expected restart confirmation output, got: %q", out)
	}
}

func TestRunTUI_ServerMenu_ReloadStays(t *testing.T) {
	origRunField := runField
	defer func() { runField = origRunField }()

	capture := captureTUIStdout(t)

	step := 0
	runField = func(_ context.Context, f huh.Field) error {
		f.WithKeyMap(huh.NewDefaultKeyMap())
		step++
		switch step {
		case 1:
			selectServerInRoot(f, 1)
			return nil
		case 2:
			selectServerAction(f, 2) // "Reload Config"
			return nil
		case 3:
			// Reload is non-destructive: stay in the submenu. ESC = Back.
			return huh.ErrUserAborted
		case 4:
			// Root: ESC exits.
			return huh.ErrUserAborted
		default:
			t.Fatalf("unexpected step %d", step)
			return nil
		}
	}

	mock := &mockTmuxClient{sessions: []tmux.Session{{Name: "worker1", Windows: 1}}}
	if err := RunTUI(t.Context(), mock); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if step != 4 {
		t.Fatalf("expected 4 steps, got %d", step)
	}
	if !mock.reloadedConfig {
		t.Fatal("expected ReloadConfig to run without confirmation")
	}
	if out := capture(); !bytes.Contains([]byte(out), []byte("Tmux config reloaded")) {
		t.Errorf("expected reload output, got: %q", out)
	}
}
