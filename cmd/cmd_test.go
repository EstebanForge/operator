// Package cmd contains unit tests for operator CLI commands.
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/EstebanForge/operator/pkg/tmux"
)

type mockTmuxClient struct {
	sessions []tmux.Session
	listErr  error
	hasErr   error
	newErr   error
	killErr  error
	peekErr  error
	sendErr  error
	docRes   *tmux.DoctorResult
	docErr   error

	lastSentPayload string
	lastSentEnter   bool
	lastSentRaw     bool
}

func (m *mockTmuxClient) ListSessions(_ context.Context) ([]tmux.Session, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.sessions, nil
}

func (m *mockTmuxClient) HasSession(_ context.Context, name string) (bool, error) {
	if m.hasErr != nil {
		return false, m.hasErr
	}
	for _, s := range m.sessions {
		if s.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockTmuxClient) NewSession(_ context.Context, name, dir, _ string, detached bool) (*tmux.Session, error) {
	if m.newErr != nil {
		return nil, m.newErr
	}
	s := tmux.Session{
		Name:       name,
		Windows:    1,
		CreatedAt:  "2026-09-05T19:30:00Z",
		IsAttached: !detached,
		Path:       dir,
	}
	m.sessions = append(m.sessions, s)
	return &s, nil
}

func (m *mockTmuxClient) Attach(_ context.Context, _ string) error {
	return nil
}

func (m *mockTmuxClient) Kill(_ context.Context, name string, all bool) error {
	if m.killErr != nil {
		return m.killErr
	}
	if all {
		m.sessions = nil
		return nil
	}
	for i, s := range m.sessions {
		if s.Name == name {
			m.sessions = append(m.sessions[:i], m.sessions[i+1:]...)
			return nil
		}
	}
	return tmux.ErrNotFound
}

func (m *mockTmuxClient) CapturePane(_ context.Context, _ string, _ int) (string, error) {
	if m.peekErr != nil {
		return "", m.peekErr
	}
	return "mock line 1\nmock line 2\n", nil
}

func (m *mockTmuxClient) SendKeys(_ context.Context, _, payload string, enter bool, raw bool) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.lastSentPayload = payload
	m.lastSentEnter = enter
	m.lastSentRaw = raw
	return nil
}

func (m *mockTmuxClient) Doctor(_ context.Context) (*tmux.DoctorResult, error) {
	if m.docErr != nil {
		return nil, m.docErr
	}
	if m.docRes != nil {
		return m.docRes, nil
	}
	return &tmux.DoctorResult{
		TmuxInstalled: true,
		TmuxPath:      "/usr/bin/tmux",
		TmuxVersion:   "tmux 3.4",
		ServerRunning: true,
	}, nil
}

func TestResolveExitCode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil error", nil, ExitSuccess},
		{"not found error", tmux.ErrNotFound, ExitNotFound},
		{"wrapped not found error", errors.Join(errors.New("wrapper"), tmux.ErrNotFound), ExitNotFound},
		{"conflict error", tmux.ErrConflict, ExitConflict},
		{"validation error", tmux.ErrValidation, ExitValidation},
		{"not a tty error", tmux.ErrNotATTY, ExitValidation},
		{"tmux missing / daemon error", tmux.ErrTmuxMissing, ExitDaemon},
		{"generic error", errors.New("something bad"), ExitDaemon},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveExitCode(tt.err)
			if got != tt.want {
				t.Errorf("ResolveExitCode(%v) = %d; want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestCommandsWithMockClient(t *testing.T) {
	origClient := client
	defer func() { client = origClient }()

	mock := &mockTmuxClient{
		sessions: []tmux.Session{
			{
				Name:       "backup-worker",
				Windows:    1,
				CreatedAt:  "2026-09-05T19:30:00Z",
				IsAttached: false,
				Path:       "/srv/storage",
			},
		},
	}
	SetClient(mock)

	t.Run("ls json format", func(t *testing.T) {
		jsonFlag = true
		defer func() { jsonFlag = false }()

		// Capture stdout
		r, w, _ := os.Pipe()
		oldStdout := os.Stdout
		os.Stdout = w

		lsCmd.Run(lsCmd, []string{})

		_ = w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		_ = r.Close()

		var sessions []tmux.Session
		if err := json.Unmarshal(buf.Bytes(), &sessions); err != nil {
			t.Fatalf("failed to unmarshal JSON: %s, output was: %s", err, buf.String())
		}
		if len(sessions) != 1 || sessions[0].Name != "backup-worker" {
			t.Errorf("unexpected sessions output: %+v", sessions)
		}
	})

	t.Run("doctor json format", func(t *testing.T) {
		jsonFlag = true
		defer func() { jsonFlag = false }()

		r, w, _ := os.Pipe()
		oldStdout := os.Stdout
		os.Stdout = w

		doctorCmd.Run(doctorCmd, []string{})

		_ = w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		_ = r.Close()

		var res tmux.DoctorResult
		if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
			t.Fatalf("failed to unmarshal JSON: %s, output was: %s", err, buf.String())
		}
		if !res.TmuxInstalled || res.TmuxVersion != "tmux 3.4" {
			t.Errorf("unexpected doctor result: %+v", res)
		}
	})

	t.Run("doctor missing tmux json format", func(t *testing.T) {
		jsonFlag = true
		defer func() { jsonFlag = false }()

		hint := tmux.DetectInstallInfo()
		oldDocRes := mock.docRes
		mock.docRes = &tmux.DoctorResult{
			TmuxInstalled: false,
			InstallHint:   &hint,
		}
		defer func() { mock.docRes = oldDocRes }()

		r, w, _ := os.Pipe()
		oldStdout := os.Stdout
		os.Stdout = w

		doctorCmd.Run(doctorCmd, []string{})

		_ = w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		_ = r.Close()

		var res tmux.DoctorResult
		if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
			t.Fatalf("failed to unmarshal JSON: %s, output was: %s", err, buf.String())
		}
		if res.TmuxInstalled || res.InstallHint == nil || res.InstallHint.Command == "" {
			t.Errorf("expected missing tmux with install hint, got: %+v", res)
		}
	})

	t.Run("peek json format", func(t *testing.T) {
		jsonFlag = true
		peekLines = 10
		defer func() { jsonFlag = false; peekLines = 25 }()

		r, w, _ := os.Pipe()
		oldStdout := os.Stdout
		os.Stdout = w

		peekCmd.Run(peekCmd, []string{"backup-worker"})

		_ = w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		_ = r.Close()

		var res map[string]any
		if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
			t.Fatalf("failed to unmarshal JSON: %s, output was: %s", err, buf.String())
		}
		if res["session"] != "backup-worker" || res["lines_captured"].(float64) != 10 {
			t.Errorf("unexpected peek output: %+v", res)
		}
	})

	t.Run("send keys", func(t *testing.T) {
		jsonFlag = true
		sendNoEnter = false
		sendRaw = false
		defer func() { jsonFlag = false }()

		r, w, _ := os.Pipe()
		oldStdout := os.Stdout
		os.Stdout = w

		sendCmd.Run(sendCmd, []string{"backup-worker", "ls -la"})

		_ = w.Close()
		_ = r.Close()
		os.Stdout = oldStdout

		if mock.lastSentPayload != "ls -la" || !mock.lastSentEnter || mock.lastSentRaw {
			t.Errorf("send params mismatch: payload=%q enter=%v raw=%v",
				mock.lastSentPayload, mock.lastSentEnter, mock.lastSentRaw)
		}
	})

	t.Run("new auto name generation", func(t *testing.T) {
		jsonFlag = true
		newDetached = true
		defer func() { jsonFlag = false; newDetached = false }()

		r, w, _ := os.Pipe()
		oldStdout := os.Stdout
		os.Stdout = w

		// Call without passing any name argument
		newCmd.Run(newCmd, []string{})

		_ = w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		_ = r.Close()

		var res map[string]any
		if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
			t.Fatalf("failed to unmarshal JSON: %s, output was: %s", err, buf.String())
		}
		if res["status"] != "ok" || res["action"] != "create" {
			t.Fatalf("unexpected new result: %+v", res)
		}
		sessName, ok := res["session"].(string)
		if !ok || sessName == "" {
			t.Fatalf("expected non-empty session name, got: %v", res["session"])
		}
		parts := strings.Split(sessName, "-")
		if len(parts) < 3 {
			t.Errorf("expected at least 3 words in auto-generated session name, got: %s", sessName)
		}
	})
}

func TestCommandErrorHandling(t *testing.T) {
	origOsExit := osExit
	origClient := client
	origIsTerm := isTerminal
	origJsonFlag := jsonFlag
	defer func() {
		osExit = origOsExit
		client = origClient
		isTerminal = origIsTerm
		jsonFlag = origJsonFlag
	}()

	mock := &mockTmuxClient{
		sessions: []tmux.Session{{Name: "worker-1"}},
	}
	SetClient(mock)
	isTerminal = func() bool { return false } // non-interactive mode

	var lastCode int
	osExit = func(code int) {
		lastCode = code
	}

	t.Run("kill --all with session name fails validation", func(t *testing.T) {
		lastCode = -1
		killAll = true
		defer func() { killAll = false }()

		killCmd.Run(killCmd, []string{"worker-1"})
		if lastCode != ExitValidation {
			t.Errorf("expected exit code %d, got %d", ExitValidation, lastCode)
		}
	})

	t.Run("kill without name or --all in non-interactive mode fails validation", func(t *testing.T) {
		lastCode = -1
		killAll = false

		killCmd.Run(killCmd, []string{})
		if lastCode != ExitValidation {
			t.Errorf("expected exit code %d, got %d", ExitValidation, lastCode)
		}
	})

	t.Run("new interactive without detached in non-interactive terminal fails validation", func(t *testing.T) {
		lastCode = -1
		newDetached = false

		newCmd.Run(newCmd, []string{"my-new-session"})
		if lastCode != ExitValidation {
			t.Errorf("expected exit code %d, got %d", ExitValidation, lastCode)
		}
	})

	t.Run("new with invalid empty sanitized name fails validation", func(t *testing.T) {
		lastCode = -1
		newDetached = true
		defer func() { newDetached = false }()

		newCmd.Run(newCmd, []string{"!@#$%^&*()"})
		if lastCode != ExitValidation {
			t.Errorf("expected exit code %d, got %d", ExitValidation, lastCode)
		}
	})

	t.Run("HandleError in JSON mode prints JSON error contract", func(t *testing.T) {
		jsonFlag = true
		defer func() { jsonFlag = false }()

		r, w, _ := os.Pipe()
		oldStderr := os.Stderr
		os.Stderr = w

		lastCode = -1
		HandleError(tmux.ErrNotFound)

		_ = w.Close()
		os.Stderr = oldStderr

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		_ = r.Close()

		var resp ErrorResponse
		if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse JSON error: %v, got: %s", err, buf.String())
		}
		if resp.Status != "error" || resp.Code != ExitNotFound {
			t.Errorf("unexpected error response: %+v", resp)
		}
		if lastCode != ExitNotFound {
			t.Errorf("expected exit code %d, got %d", ExitNotFound, lastCode)
		}
	})

	t.Run("HandleError with nil exits 0", func(t *testing.T) {
		lastCode = -1
		HandleError(nil)
		if lastCode != ExitSuccess {
			t.Errorf("expected exit code %d, got %d", ExitSuccess, lastCode)
		}
	})
}
