package tmux

import (
	"context"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestOSClient_RealTmux_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping tmux integration tests in short mode")
	}

	tmuxPath, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux binary not installed, skipping integration tests")
	}

	client := NewOSClient()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()

	t.Run("Doctor reporting with real tmux", func(t *testing.T) {
		doc, err := client.Doctor(ctx)
		if err != nil {
			t.Fatalf("Doctor returned unexpected error: %v", err)
		}
		if !doc.TmuxInstalled {
			t.Fatalf("expected TmuxInstalled to be true for %s", tmuxPath)
		}
		if doc.TmuxVersion == "" {
			t.Errorf("expected TmuxVersion to be non-empty")
		}
		if doc.TmuxPath == "" {
			t.Errorf("expected TmuxPath to be non-empty")
		}
	})

	t.Run("HasSession with non-existent session", func(t *testing.T) {
		nonExistent := "operator-test-nonexistent-" + GenerateSessionName()
		exists, err := client.HasSession(ctx, nonExistent)
		if err != nil {
			t.Fatalf("unexpected error checking non-existent session: %v", err)
		}
		if exists {
			t.Fatalf("expected session %q to not exist", nonExistent)
		}
	})

	t.Run("NewWindow creates tab in session", func(t *testing.T) {
		sessName := "operator-test-" + GenerateSessionName()
		defer func() { _ = client.Kill(ctx, sessName, false) }()

		if _, err := client.NewSession(ctx, sessName, "", "", true); err != nil {
			t.Fatalf("failed to create base session: %v", err)
		}

		res, err := client.NewWindow(ctx, sessName, "tabcheck", "/tmp")
		if err != nil {
			t.Fatalf("failed to create window: %v", err)
		}
		if res.Name != "tabcheck" || res.Directory != "/tmp" {
			t.Errorf("unexpected window result: %+v", res)
		}
		if !strings.HasPrefix(res.Window, "@") {
			t.Errorf("expected window id like '@1', got %q", res.Window)
		}

		// Base-index agnostic: the second tab must sit above the first.
		res2, err := client.NewWindow(ctx, sessName, "tabcheck2", "/tmp")
		if err != nil {
			t.Fatalf("failed to create second window: %v", err)
		}
		if res2.Index <= res.Index {
			t.Errorf("expected increasing window index, got %d then %d", res.Index, res2.Index)
		}

		if _, err := client.NewWindow(ctx, "operator-test-missing-"+GenerateSessionName(), "x", ""); err == nil {
			t.Errorf("expected error for missing session")
		}
	})

	t.Run("Kill non-existent session returns ErrNotFound", func(t *testing.T) {
		nonExistent := "operator-test-nonexistent-" + GenerateSessionName()
		err := client.Kill(ctx, nonExistent, false)
		if err == nil {
			t.Fatalf("expected Kill on non-existent session to fail")
		}
		if !strings.Contains(err.Error(), "session not found") {
			t.Errorf("expected error to contain 'session not found', got: %v", err)
		}
	})

	t.Run("Detached session lifecycle", func(t *testing.T) {
		sessName := "operator-test-" + GenerateSessionName()

		// Ensure cleanup if test fails mid-way
		defer func() {
			_ = client.Kill(ctx, sessName, false)
		}()

		sess, err := client.NewSession(ctx, sessName, "", "", true)
		if err != nil {
			t.Fatalf("failed to create real detached session: %v", err)
		}
		if sess.Name != sessName {
			t.Errorf("expected created session name %q, got %q", sessName, sess.Name)
		}

		// HasSession should return true
		exists, err := client.HasSession(ctx, sessName)
		if err != nil || !exists {
			t.Fatalf("expected HasSession to be true, got exists=%v err=%v", exists, err)
		}

		// ListSessions should include sessName
		sessions, err := client.ListSessions(ctx)
		if err != nil {
			t.Fatalf("failed to list sessions: %v", err)
		}
		found := slices.ContainsFunc(sessions, func(s Session) bool {
			return s.Name == sessName
		})
		if !found {
			t.Fatalf("expected ListSessions to contain %q, got: %+v", sessName, sessions)
		}

		// SendKeys
		err = client.SendKeys(ctx, sessName, "echo test", true, false)
		if err != nil {
			t.Errorf("failed to send keys: %v", err)
		}

		// CapturePane
		lines, err := client.CapturePane(ctx, sessName, 10)
		if err != nil {
			t.Errorf("failed to capture pane: %v", err)
		}
		if len(lines) == 0 {
			t.Logf("captured 0 lines (pane may be empty initially)")
		}

		// Kill session
		err = client.Kill(ctx, sessName, false)
		if err != nil {
			t.Fatalf("failed to kill session: %v", err)
		}

		// Verify session is gone
		exists, err = client.HasSession(ctx, sessName)
		if err != nil {
			t.Fatalf("error checking session after kill: %v", err)
		}
		if exists {
			t.Fatalf("expected session %q to be gone after kill", sessName)
		}
	})
}
