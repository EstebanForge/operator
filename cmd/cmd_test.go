// Package cmd contains unit tests for operator CLI commands.
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EstebanForge/operator/pkg/tmux"
)

type mockTmuxClient struct {
	sessions  []tmux.Session
	listErr   error
	hasErr    error
	newErr    error
	newWinErr error
	killErr   error
	peekErr   error
	sendErr   error
	docRes    *tmux.DoctorResult
	docErr    error

	lastSentPayload string
	lastSentEnter   bool
	lastSentRaw     bool
	lastWindow      tmux.WindowResult
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

func (m *mockTmuxClient) NewWindow(_ context.Context, _, name, dir string) (*tmux.WindowResult, error) {
	if m.newWinErr != nil {
		return nil, m.newWinErr
	}
	if name == "" {
		name = "bash"
	}
	m.lastWindow = tmux.WindowResult{Window: "@9", Index: 9, Name: name, Directory: dir}
	res := m.lastWindow
	return &res, nil
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
		// The mock capture returns two lines while -l requests 10: the contract
		// reports the actual line count, not the request.
		if res["session"] != "backup-worker" || res["lines_captured"].(float64) != 2 {
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
	origJSONFlag := jsonFlag
	defer func() {
		osExit = origOsExit
		client = origClient
		isTerminal = origIsTerm
		jsonFlag = origJSONFlag
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

func TestDetachAndClosedHints(t *testing.T) {
	t.Parallel()
	got := detachHint("work")
	if !strings.Contains(got, "Detached. 'work' is still running.") ||
		!strings.Contains(got, "operator join work") ||
		!strings.Contains(got, "operator kill work") {
		t.Errorf("detachHint missing detach/reattach/kill guidance: %q", got)
	}
	if got := closedHint("work"); got != "Session 'work' closed.\n" {
		t.Errorf("unexpected closedHint: %q", got)
	}
}

func TestPrintSessionExitHint(t *testing.T) {
	origClient := client
	origJSON := jsonFlag
	defer func() { client = origClient; jsonFlag = origJSON }()

	capture := func() string {
		r, w, _ := os.Pipe()
		oldStderr := os.Stderr
		os.Stderr = w
		printSessionExitHint(context.Background(), client, "work")
		_ = w.Close()
		os.Stderr = oldStderr
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		_ = r.Close()
		return buf.String()
	}

	t.Run("alive session prints detach hint", func(t *testing.T) {
		t.Setenv("TMUX", "")
		jsonFlag = false
		client = &mockTmuxClient{sessions: []tmux.Session{{Name: "work"}}}
		if out := capture(); !strings.Contains(out, "Detached. 'work' is still running.") {
			t.Errorf("expected detach hint, got %q", out)
		}
	})

	t.Run("gone session prints closed hint", func(t *testing.T) {
		t.Setenv("TMUX", "")
		jsonFlag = false
		client = &mockTmuxClient{}
		if out := capture(); out != "Session 'work' closed.\n" {
			t.Errorf("expected closed hint, got %q", out)
		}
	})

	t.Run("json mode prints nothing", func(t *testing.T) {
		t.Setenv("TMUX", "")
		jsonFlag = true
		client = &mockTmuxClient{sessions: []tmux.Session{{Name: "work"}}}
		if out := capture(); out != "" {
			t.Errorf("expected no hint in json mode, got %q", out)
		}
	})

	t.Run("inside tmux prints nothing", func(t *testing.T) {
		t.Setenv("TMUX", "/tmp/tmux-0/default,123,0")
		jsonFlag = false
		client = &mockTmuxClient{sessions: []tmux.Session{{Name: "work"}}}
		if out := capture(); out != "" {
			t.Errorf("expected no hint inside tmux, got %q", out)
		}
	})
}

func TestTabCommand(t *testing.T) {
	origOsExit := osExit
	origClient := client
	origIsTerm := isTerminal
	origJSON := jsonFlag
	origTabDir, origTabName := tabDir, tabName
	defer func() {
		osExit = origOsExit
		client = origClient
		isTerminal = origIsTerm
		jsonFlag = origJSON
		tabDir, tabName = origTabDir, origTabName
	}()

	var lastCode int
	osExit = func(code int) { lastCode = code }
	isTerminal = func() bool { return false }

	t.Run("non-interactive without session fails validation", func(t *testing.T) {
		lastCode = -1
		tabDir, tabName = "", ""
		SetClient(&mockTmuxClient{})
		tabCmd.Run(tabCmd, []string{})
		if lastCode != ExitValidation {
			t.Errorf("expected exit code %d, got %d", ExitValidation, lastCode)
		}
	})

	t.Run("json result carries window details", func(t *testing.T) {
		lastCode = -1
		jsonFlag = true
		// The resolver validates the flag on disk, so use a real directory.
		flagDir := t.TempDir()
		tabDir, tabName = flagDir, "build"
		mock := &mockTmuxClient{sessions: []tmux.Session{{Name: "work"}}}
		SetClient(mock)

		r, w, _ := os.Pipe()
		oldStdout := os.Stdout
		os.Stdout = w
		tabCmd.Run(tabCmd, []string{"work"})
		_ = w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		_ = r.Close()

		var res map[string]any
		if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
			t.Fatalf("failed to unmarshal JSON: %s, output was: %s", err, buf.String())
		}
		if res["status"] != "ok" || res["action"] != "tab" || res["session"] != "work" {
			t.Fatalf("unexpected tab result: %+v", res)
		}
		details := res["details"].(map[string]any)
		if details["window"] != "@9" || details["index"] != float64(9) || details["name"] != "build" || details["directory"] != flagDir {
			t.Errorf("unexpected details: %+v", details)
		}
		if lastCode != -1 {
			t.Errorf("success path must not exit, got code %d", lastCode)
		}
	})

	t.Run("missing session maps to not found", func(t *testing.T) {
		lastCode = -1
		jsonFlag = false
		tabDir, tabName = "", ""
		SetClient(&mockTmuxClient{newWinErr: fmt.Errorf("%w: session 'ghost'", tmux.ErrNotFound)})

		tabCmd.Run(tabCmd, []string{"ghost"})
		if lastCode != ExitNotFound {
			t.Errorf("expected exit code %d, got %d", ExitNotFound, lastCode)
		}
	})

	t.Run("invalid directory maps to validation", func(t *testing.T) {
		lastCode = -1
		jsonFlag = false
		tabDir, tabName = "/no/such/dir", ""
		SetClient(&mockTmuxClient{newWinErr: fmt.Errorf("%w: directory not found: /no/such/dir", tmux.ErrValidation)})

		tabCmd.Run(tabCmd, []string{"work"})
		if lastCode != ExitValidation {
			t.Errorf("expected exit code %d, got %d", ExitValidation, lastCode)
		}
	})
}

func TestResolveTabDir(t *testing.T) {
	origClient := client
	defer func() { client = origClient }()

	tmp := t.TempDir()
	sessDir := filepath.Join(tmp, "sessdir")
	if err := os.Mkdir(sessDir, 0o755); err != nil {
		t.Fatal(err)
	}
	goneDir := filepath.Join(tmp, "gone")

	SetClient(&mockTmuxClient{sessions: []tmux.Session{{Name: "work", Path: sessDir}}})
	ctx := context.Background()

	if got := resolveTabDir(ctx, "work", sessDir, tmp); got != tmp {
		t.Errorf("existing flag must win: got %q", got)
	}
	if got := resolveTabDir(ctx, "work", tmp, goneDir); got != tmp {
		t.Errorf("expected valid hint over listing, got %q", got)
	}
	if got := resolveTabDir(ctx, "work", goneDir, goneDir); got != sessDir {
		t.Errorf("expected session listing fallback, got %q", got)
	}
	if got := resolveTabDir(ctx, "ghost", goneDir, goneDir); got == "" || got == goneDir {
		t.Errorf("expected cwd fallback for unknown session, got %q", got)
	}
}

func TestOprOwnership(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	foreignFile := filepath.Join(dir, "foreign-file")
	if err := os.WriteFile(foreignFile, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	foreignLink := filepath.Join(dir, "foreign-link")
	if err := os.Symlink("/usr/bin/something-else", foreignLink); err != nil {
		t.Fatal(err)
	}
	realBin := filepath.Join(dir, "operator")
	if err := os.WriteFile(realBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ownedRelative := filepath.Join(dir, "owned-relative")
	if err := os.Symlink("operator", ownedRelative); err != nil {
		t.Fatal(err)
	}
	ownedAbsolute := filepath.Join(dir, "owned-absolute")
	if err := os.Symlink(realBin, ownedAbsolute); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		path       string
		wantExists bool
		wantOwned  bool
	}{
		{"absent entry", filepath.Join(dir, "opr"), false, false},
		{"foreign regular file", foreignFile, true, false},
		{"foreign symlink", foreignLink, true, false},
		{"owned relative symlink", ownedRelative, true, true},
		{"owned absolute symlink", ownedAbsolute, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			exists, owned := oprOwnership(tt.path, "operator", realBin)
			if exists != tt.wantExists || owned != tt.wantOwned {
				t.Errorf("oprOwnership(%q) = (%v, %v); want (%v, %v)", tt.path, exists, owned, tt.wantExists, tt.wantOwned)
			}
		})
	}
}

// runSetupWithFakeBinary points osExecutable at a fake binary inside a fresh
// temp dir and returns the dir path.
func runSetupWithFakeBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "operator")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	osExecutable = func() (string, error) { return fakeBin, nil }
	return dir
}

func TestSetupCommandRefusesForeignOpr(t *testing.T) {
	origExec := osExecutable
	origOsExit := osExit
	origJSON := jsonFlag
	defer func() { osExecutable = origExec; osExit = origOsExit; jsonFlag = origJSON }()
	dir := runSetupWithFakeBinary(t)

	foreignOpr := filepath.Join(dir, "opr")
	if err := os.WriteFile(foreignOpr, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	var lastCode int
	osExit = func(code int) { lastCode = code }
	jsonFlag = false

	setupCmd.Run(setupCmd, []string{})

	if lastCode != ExitConflict {
		t.Errorf("expected exit code %d, got %d", ExitConflict, lastCode)
	}
	content, err := os.ReadFile(foreignOpr)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "keep me" {
		t.Errorf("foreign opr was modified: %q", content)
	}
}

func TestSetupCommandRefreshesOwnedSymlink(t *testing.T) {
	origExec := osExecutable
	origOsExit := osExit
	origJSON := jsonFlag
	defer func() { osExecutable = origExec; osExit = origOsExit; jsonFlag = origJSON }()
	dir := runSetupWithFakeBinary(t)

	oprPath := filepath.Join(dir, "opr")
	if err := os.Symlink("operator", oprPath); err != nil {
		t.Fatal(err)
	}

	var lastCode int
	osExit = func(code int) { lastCode = code }
	jsonFlag = false

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w
	setupCmd.Run(setupCmd, []string{})
	_ = w.Close()
	os.Stdout = oldStdout
	_ = r.Close()

	if lastCode != ExitSuccess {
		t.Errorf("expected exit code %d, got %d", ExitSuccess, lastCode)
	}
	link, err := os.Readlink(oprPath)
	if err != nil {
		t.Fatalf("expected managed symlink to be recreated: %v", err)
	}
	if link != "operator" {
		t.Errorf("expected symlink target 'operator', got %q", link)
	}
}

func TestSendJoinsMultiWordPayload(t *testing.T) {
	origClient := client
	origJSON := jsonFlag
	defer func() { client = origClient; jsonFlag = origJSON }()

	mock := &mockTmuxClient{sessions: []tmux.Session{{Name: "web"}}}
	SetClient(mock)
	jsonFlag = true

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w
	sendCmd.Run(sendCmd, []string{"web", "echo", "hello", "world"})
	_ = w.Close()
	os.Stdout = oldStdout
	_ = r.Close()

	if mock.lastSentPayload != "echo hello world" {
		t.Errorf("expected joined payload %q, got %q", "echo hello world", mock.lastSentPayload)
	}
}

func TestNewExitedSessionIsSuccess(t *testing.T) {
	origClient := client
	origTerm := isTerminal
	origDetached := newDetached
	origJSON := jsonFlag
	defer func() {
		client = origClient
		isTerminal = origTerm
		newDetached = origDetached
		jsonFlag = origJSON
	}()

	mock := &mockTmuxClient{
		newErr: fmt.Errorf("%w: session 'run-and-done' exited", tmux.ErrSessionExited),
	}
	SetClient(mock)
	isTerminal = func() bool { return true }
	newDetached = false
	jsonFlag = true

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w
	newCmd.Run(newCmd, []string{"run-and-done"})
	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	_ = r.Close()

	var res map[string]any
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("failed to unmarshal JSON: %s, output was: %s", err, buf.String())
	}
	if res["status"] != "ok" {
		t.Errorf("expected status ok, got: %+v", res)
	}
	details, ok := res["details"].(map[string]any)
	if !ok || details["exited"] != true {
		t.Errorf("expected details.exited true, got: %+v", res["details"])
	}
}
