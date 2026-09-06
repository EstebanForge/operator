package cmd

import (
	"context"
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

func TestRunTUI_AttachEscapeGoesBackToRoot(t *testing.T) {
	origRunField := runField
	defer func() { runField = origRunField }()

	step := 0
	runField = func(_ context.Context, f huh.Field) error {
		f.WithKeyMap(huh.NewDefaultKeyMap())
		step++
		switch step {
		case 1:
			// Root menu: select "attach"
			sel, ok := f.(*huh.Select[string])
			if !ok {
				t.Fatalf("expected *huh.Select[string] at step 1")
			}
			sel.Init()
			sel.Update(tea.KeyMsg{Type: tea.KeyEnter}) // selects "attach" (first item)
			return nil
		case 2:
			// Submenu: Attach session selection -> press ESC
			return huh.ErrUserAborted
		case 3:
			// Returned back to root menu! Now press ESC to exit
			return huh.ErrUserAborted
		default:
			t.Fatalf("unexpected step %d", step)
			return nil
		}
	}

	mock := &mockTmuxClient{
		sessions: []tmux.Session{
			{Name: "sess1", Windows: 1},
		},
	}

	err := RunTUI(t.Context(), mock)
	if err != nil {
		t.Fatalf("expected RunTUI to return nil, got: %v", err)
	}
	if step != 3 {
		t.Fatalf("expected 3 steps (root -> attach [ESC] -> root [ESC]), got %d", step)
	}
}

func TestRunTUI_PeekEscapeGoesBackToRoot(t *testing.T) {
	origRunField := runField
	defer func() { runField = origRunField }()

	step := 0
	runField = func(_ context.Context, f huh.Field) error {
		f.WithKeyMap(huh.NewDefaultKeyMap())
		step++
		switch step {
		case 1:
			// Root menu: navigate to "peek" (down twice: attach -> new -> peek)
			sel, ok := f.(*huh.Select[string])
			if !ok {
				t.Fatalf("expected *huh.Select[string] at step 1")
			}
			sel.Init()
			sel.Update(tea.KeyMsg{Type: tea.KeyDown})
			sel.Update(tea.KeyMsg{Type: tea.KeyDown})
			sel.Update(tea.KeyMsg{Type: tea.KeyEnter})
			return nil
		case 2:
			// Submenu: Peek session selection -> press ESC
			return huh.ErrUserAborted
		case 3:
			// Returned to root menu -> press ESC to exit
			return huh.ErrUserAborted
		default:
			t.Fatalf("unexpected step %d", step)
			return nil
		}
	}

	mock := &mockTmuxClient{
		sessions: []tmux.Session{
			{Name: "sess1", Windows: 1},
		},
	}

	err := RunTUI(t.Context(), mock)
	if err != nil {
		t.Fatalf("expected RunTUI to return nil, got: %v", err)
	}
	if step != 3 {
		t.Fatalf("expected 3 steps, got %d", step)
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
				// Root menu: select "new" (when 0 sessions, option 0 is "new")
				sel := f.(*huh.Select[string])
				sel.Update(tea.KeyMsg{Type: tea.KeyEnter})
				return nil
			case 2:
				// Submenu "new" Step 1: Session name input -> press ESC
				return huh.ErrUserAborted
			case 3:
				// Returned to root -> press ESC to exit
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

	// Test: Root -> New -> Step 1 (enter) -> Step 2 (confirm) -> ESC -> Step 1 (Session name) -> ESC -> Root -> ESC
	t.Run("ESC at step 2 goes back to step 1", func(t *testing.T) {
		step := 0
		runField = func(_ context.Context, f huh.Field) error {
			step++
			switch step {
			case 1:
				// Root menu: select "new"
				sel := f.(*huh.Select[string])
				sel.Update(tea.KeyMsg{Type: tea.KeyEnter})
				return nil
			case 2:
				// Submenu "new" Step 1: Session name input -> press Enter
				input := f.(*huh.Input)
				input.Update(tea.KeyMsg{Type: tea.KeyEnter})
				return nil
			case 3:
				// Submenu "new" Step 2: Confirm attach -> press ESC (goes back to Step 1)
				return huh.ErrUserAborted
			case 4:
				// Back at Submenu "new" Step 1: Session name input -> press ESC (goes back to Root)
				return huh.ErrUserAborted
			case 5:
				// Back at Root menu -> press ESC to exit
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

	// Test: Root -> Kill -> Step 1 (Select session) -> ESC -> Root -> ESC
	t.Run("ESC at step 1 goes back to root", func(t *testing.T) {
		step := 0
		runField = func(_ context.Context, f huh.Field) error {
			f.WithKeyMap(huh.NewDefaultKeyMap())
			step++
			switch step {
			case 1:
				// Root: navigate to "kill" (down 3 times: attach -> new -> peek -> kill)
				sel := f.(*huh.Select[string])
				sel.Init()
				sel.Update(tea.KeyMsg{Type: tea.KeyDown})
				sel.Update(tea.KeyMsg{Type: tea.KeyDown})
				sel.Update(tea.KeyMsg{Type: tea.KeyDown})
				sel.Update(tea.KeyMsg{Type: tea.KeyEnter})
				return nil
			case 2:
				// Submenu "kill" Step 1: Select Session -> press ESC
				return huh.ErrUserAborted
			case 3:
				// Back at Root menu -> press ESC to exit
				return huh.ErrUserAborted
			default:
				t.Fatalf("unexpected step %d", step)
				return nil
			}
		}

		mock := &mockTmuxClient{
			sessions: []tmux.Session{{Name: "worker1", Windows: 1}},
		}
		err := RunTUI(t.Context(), mock)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if step != 3 {
			t.Fatalf("expected 3 steps, got %d", step)
		}
	})

	// Test: Root -> Kill -> Step 1 (select) -> Step 2 (confirm) -> ESC -> Step 1 -> ESC -> Root -> ESC
	t.Run("ESC at step 2 goes back to step 1", func(t *testing.T) {
		step := 0
		runField = func(_ context.Context, f huh.Field) error {
			f.WithKeyMap(huh.NewDefaultKeyMap())
			step++
			switch step {
			case 1:
				// Root: select "kill"
				sel := f.(*huh.Select[string])
				sel.Init()
				sel.Update(tea.KeyMsg{Type: tea.KeyDown})
				sel.Update(tea.KeyMsg{Type: tea.KeyDown})
				sel.Update(tea.KeyMsg{Type: tea.KeyDown})
				sel.Update(tea.KeyMsg{Type: tea.KeyEnter})
				return nil
			case 2:
				// Submenu "kill" Step 1: Select session -> press Enter
				sel := f.(*huh.Select[string])
				sel.Init()
				sel.Update(tea.KeyMsg{Type: tea.KeyEnter})
				return nil
			case 3:
				// Submenu "kill" Step 2: Confirm kill -> press ESC (goes back to Step 1)
				return huh.ErrUserAborted
			case 4:
				// Back at Submenu "kill" Step 1 -> press ESC (goes back to Root)
				return huh.ErrUserAborted
			case 5:
				// Back at Root menu -> press ESC to exit
				return huh.ErrUserAborted
			default:
				t.Fatalf("unexpected step %d", step)
				return nil
			}
		}

		mock := &mockTmuxClient{
			sessions: []tmux.Session{{Name: "worker1", Windows: 1}},
		}
		err := RunTUI(t.Context(), mock)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if step != 5 {
			t.Fatalf("expected 5 steps, got %d", step)
		}
	})

	// Test: Rejecting confirm ("No") in kill goes back to Step 1
	t.Run("Rejecting confirm in kill goes back to step 1", func(t *testing.T) {
		step := 0
		runField = func(_ context.Context, f huh.Field) error {
			f.WithKeyMap(huh.NewDefaultKeyMap())
			step++
			switch step {
			case 1:
				// Root: select "kill"
				sel := f.(*huh.Select[string])
				sel.Init()
				sel.Update(tea.KeyMsg{Type: tea.KeyDown})
				sel.Update(tea.KeyMsg{Type: tea.KeyDown})
				sel.Update(tea.KeyMsg{Type: tea.KeyDown})
				sel.Update(tea.KeyMsg{Type: tea.KeyEnter})
				return nil
			case 2:
				// Submenu "kill" Step 1: Select session -> press Enter
				sel := f.(*huh.Select[string])
				sel.Init()
				sel.Update(tea.KeyMsg{Type: tea.KeyEnter})
				return nil
			case 3:
				// Submenu "kill" Step 2: Confirm kill -> choose No ('n')
				confirm := f.(*huh.Confirm)
				confirm.Init()
				confirm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
				return nil
			case 4:
				// Back at Submenu "kill" Step 1 -> press ESC (goes back to Root)
				return huh.ErrUserAborted
			case 5:
				// Back at Root menu -> press ESC to exit
				return huh.ErrUserAborted
			default:
				t.Fatalf("unexpected step %d", step)
				return nil
			}
		}

		mock := &mockTmuxClient{
			sessions: []tmux.Session{{Name: "worker1", Windows: 1}},
		}
		err := RunTUI(t.Context(), mock)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if step != 5 {
			t.Fatalf("expected 5 steps, got %d", step)
		}
	})
}
