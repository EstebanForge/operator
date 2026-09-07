// Package tmux provides a client abstraction for managing native tmux sessions.
package tmux

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/mattn/go-isatty"
)

var (
	// ErrNotFound indicates the target tmux session does not exist.
	ErrNotFound = errors.New("session not found")
	// ErrConflict indicates the session name is already taken.
	ErrConflict = errors.New("session already exists")
	// ErrValidation indicates invalid parameters or invalid session names.
	ErrValidation = errors.New("invalid session name or argument")
	// ErrNotATTY indicates an interactive terminal is required but not present.
	ErrNotATTY = errors.New("terminal required for attach")
	// ErrTmuxMissing indicates the tmux executable was not found in PATH.
	ErrTmuxMissing = errors.New("tmux binary not found in PATH")
	// ErrSessionExited indicates an interactive session was terminated upon exiting tmux.
	ErrSessionExited = fmt.Errorf("%w: session exited", ErrNotFound)
)

// Session represents a running tmux session.
type Session struct {
	Name       string `json:"name"`
	Windows    int    `json:"windows"`
	CreatedAt  string `json:"created_at"`
	IsAttached bool   `json:"is_attached"`
	Path       string `json:"path"`
}

// DoctorResult represents tmux diagnostic information.
type DoctorResult struct {
	TmuxInstalled bool         `json:"tmux_installed"`
	TmuxPath      string       `json:"tmux_path"`
	TmuxVersion   string       `json:"tmux_version"`
	ServerRunning bool         `json:"server_running"`
	InstallHint   *InstallInfo `json:"install_hint,omitempty"`
}

// windowFormat is the tab-delimited -P -F output contract for new-window.
const windowFormat = "#{window_id}\t#{window_index}\t#{window_name}\t#{pane_current_path}"

// WindowResult describes a tmux window (tab) created by NewWindow.
type WindowResult struct {
	Window    string `json:"window"`    // tmux window id, e.g. "@3"
	Index     int    `json:"index"`     // window index inside the session
	Name      string `json:"name"`      // window name (shell name when unset)
	Directory string `json:"directory"` // start directory of the window's pane
}

// Client defines operations against the tmux server.
type Client interface {
	ListSessions(ctx context.Context) ([]Session, error)
	HasSession(ctx context.Context, name string) (bool, error)
	NewSession(ctx context.Context, name, dir, cmdStr string, detached bool) (*Session, error)
	NewWindow(ctx context.Context, session, name, dir string) (*WindowResult, error)
	Attach(ctx context.Context, name string) error
	Kill(ctx context.Context, name string, all bool) error
	CapturePane(ctx context.Context, name string, lines int) (string, error)
	SendKeys(ctx context.Context, name, payload string, enter bool, raw bool) error
	Doctor(ctx context.Context) (*DoctorResult, error)
}

// OSClient interacts with tmux using os/exec.
type OSClient struct {
	tmuxPath string
}

// NewOSClient constructs a new OSClient.
func NewOSClient() *OSClient {
	path, _ := exec.LookPath("tmux") //nolint:errcheck // lazily resolved in run
	return &OSClient{tmuxPath: path}
}

// sessionTarget prefixes a session name with '=' so tmux matches it exactly.
// Bare names fall back to unambiguous-prefix matching, so '-t work' would
// resolve to 'worker' and kill the wrong session.
func sessionTarget(name string) string {
	return "=" + name
}

// paneTarget targets the active pane of a session exactly. Pane commands
// (capture-pane, send-keys) reject a bare '=name' as a pane target, so the
// trailing colon scopes the empty window/pane part to the exact session.
func paneTarget(name string) string {
	return "=" + name + ":"
}

// SanitizeSessionName converts whitespace to '-' and removes characters that are not ASCII alphanumeric, dash, or underscore.
func SanitizeSessionName(name string) string {
	var sb strings.Builder
	sb.Grow(len(name))
	for _, r := range name {
		if unicode.IsSpace(r) {
			sb.WriteRune('-')
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func (c *OSClient) ensureTmuxPath() error {
	if c.tmuxPath != "" {
		return nil
	}
	path, err := exec.LookPath("tmux")
	if err != nil {
		return ErrTmuxMissing
	}
	c.tmuxPath = path
	return nil
}

func (c *OSClient) run(ctx context.Context, args ...string) ([]byte, error) {
	if err := c.ensureTmuxPath(); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, c.tmuxPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		errStr := strings.TrimSpace(stderr.String())
		if errStr != "" {
			return nil, fmt.Errorf("%s: %w", errStr, err)
		}
		return nil, err
	}
	return stdout.Bytes(), nil
}

// ParseSessionList parses the tab-delimited tmux list-sessions output.
// Trims only line endings: a trailing tab is significant (empty pane path).
func ParseSessionList(output string) []Session {
	lines := strings.Split(strings.TrimRight(output, "\r\n"), "\n")
	sessions := make([]Session, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 5 {
			continue
		}

		windows, _ := strconv.Atoi(parts[1])                //nolint:errcheck // fallback to 0
		createdSec, _ := strconv.ParseInt(parts[2], 10, 64) //nolint:errcheck // fallback to 0
		createdAt := time.Unix(createdSec, 0).UTC().Format(time.RFC3339)
		attachedCount, _ := strconv.Atoi(parts[3]) //nolint:errcheck // fallback to 0

		sessions = append(sessions, Session{
			Name:       parts[0],
			Windows:    windows,
			CreatedAt:  createdAt,
			IsAttached: attachedCount > 0,
			Path:       parts[4],
		})
	}
	return sessions
}

// ListSessions queries active tmux sessions using tab-delimited formatting.
func (c *OSClient) ListSessions(ctx context.Context) ([]Session, error) {
	format := "#{session_name}\t#{session_windows}\t#{session_created}\t#{session_attached}\t#{pane_current_path}"
	out, err := c.run(ctx, "list-sessions", "-F", format)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "no server running") ||
			strings.Contains(errMsg, "no sessions") ||
			strings.Contains(errMsg, "error connecting to") {
			return []Session{}, nil
		}
		return nil, fmt.Errorf("list-sessions failed: %w", err)
	}

	return ParseSessionList(string(out)), nil
}

// HasSession checks if a session exists.
func (c *OSClient) HasSession(ctx context.Context, name string) (bool, error) {
	_, err := c.run(ctx, "has-session", "-t", sessionTarget(name))
	if err != nil {
		errMsg := err.Error()
		// Mirror ListSessions: a dead or absent server means the session
		// cannot exist, which is not an error (ADR 005).
		if strings.Contains(errMsg, "can't find session") ||
			strings.Contains(errMsg, "no server running") ||
			strings.Contains(errMsg, "no sessions") ||
			strings.Contains(errMsg, "error connecting to") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// NewSession creates a new tmux session.
func (c *OSClient) NewSession(ctx context.Context, name, dir, cmdStr string, detached bool) (*Session, error) {
	sanitized := SanitizeSessionName(name)
	if sanitized == "" {
		return nil, fmt.Errorf("%w: session name must not be empty", ErrValidation)
	}

	exists, err := c.HasSession(ctx, sanitized)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("%w: session '%s'", ErrConflict, sanitized)
	}

	args := []string{"new-session", "-s", sanitized}
	if dir != "" {
		args = append(args, "-c", dir)
	}
	if detached {
		args = append(args, "-d")
	}
	if cmdStr != "" {
		args = append(args, cmdStr)
	}

	if detached {
		_, err := c.run(ctx, args...)
		if err != nil {
			return nil, fmt.Errorf("failed to create session: %w", err)
		}
	} else {
		if !IsTerminal() {
			return nil, ErrNotATTY
		}
		if err := c.ensureTmuxPath(); err != nil {
			return nil, err
		}
		cmd := exec.CommandContext(ctx, c.tmuxPath, args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("failed to attach to new session: %w", err)
		}
	}

	sessions, err := c.ListSessions(ctx)
	if err == nil {
		if idx := slices.IndexFunc(sessions, func(s Session) bool { return s.Name == sanitized }); idx >= 0 {
			return &sessions[idx], nil
		}
	}

	if !detached {
		// An interactive session that is no longer listed has terminated/exited.
		return nil, fmt.Errorf("%w: session '%s' exited", ErrSessionExited, sanitized)
	}

	// Detached session fallback check
	exists, hErr := c.HasSession(ctx, sanitized)
	if hErr == nil && !exists {
		return nil, fmt.Errorf("%w: session '%s' could not be found after creation", ErrNotFound, sanitized)
	}

	return &Session{
		Name:       sanitized,
		Windows:    1,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
		IsAttached: false,
		Path:       dir,
	}, nil
}

// NewWindow creates a new window (tab) in an existing session at the next
// free index. An empty name lets tmux name the window after its running
// shell; an empty dir falls back to the tmux client's working directory.
func (c *OSClient) NewWindow(ctx context.Context, session, name, dir string) (*WindowResult, error) {
	session = strings.TrimSpace(session)
	if session == "" {
		return nil, fmt.Errorf("%w: session name required", ErrValidation)
	}

	exists, err := c.HasSession(ctx, session)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("%w: session '%s'", ErrNotFound, session)
	}

	if dir != "" {
		if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
			return nil, fmt.Errorf("%w: directory not found: %s", ErrValidation, dir)
		}
	}

	// '=session:' targets the exact session with an empty window part, which
	// new-window resolves to the next free index.
	args := []string{"new-window", "-P", "-F", windowFormat, "-t", paneTarget(session)}
	if dir != "" {
		args = append(args, "-c", dir)
	}
	if name != "" {
		args = append(args, "-n", name)
	}

	out, err := c.run(ctx, args...)
	if err != nil {
		errMsg := err.Error()
		// Mirror HasSession: the server may vanish between the existence check
		// and the call (last session killed); that is not-found, not daemon.
		if strings.Contains(errMsg, "can't find session") ||
			strings.Contains(errMsg, "no server running") ||
			strings.Contains(errMsg, "no sessions") ||
			strings.Contains(errMsg, "error connecting to") {
			return nil, fmt.Errorf("%w: session '%s'", ErrNotFound, session)
		}
		return nil, fmt.Errorf("new-window failed: %w", err)
	}
	return parseWindowResult(string(out))
}

// parseWindowResult parses the tab-delimited -P -F output of new-window.
// Trims only line endings: a trailing tab is significant (empty pane path).
func parseWindowResult(output string) (*WindowResult, error) {
	parts := strings.Split(strings.TrimRight(output, "\r\n"), "\t")
	if len(parts) < 4 {
		return nil, fmt.Errorf("unexpected new-window output: %q", output)
	}
	index, _ := strconv.Atoi(parts[1]) //nolint:errcheck // fallback to 0
	return &WindowResult{
		Window:    parts[0],
		Index:     index,
		Name:      parts[2],
		Directory: parts[3],
	}, nil
}

// IsTerminal returns true if both stdin and stdout are interactive terminals.
func IsTerminal() bool {
	return (isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())) &&
		(isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()))
}

// InsideTmux reports whether the process runs inside a tmux pane.
func InsideTmux() bool {
	return os.Getenv("TMUX") != ""
}

// Attach attaches to a session or switches client if already inside tmux.
func (c *OSClient) Attach(ctx context.Context, name string) error {
	exists, err := c.HasSession(ctx, name)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%w: session '%s'", ErrNotFound, name)
	}

	if InsideTmux() {
		_, err := c.run(ctx, "switch-client", "-t", sessionTarget(name))
		return err
	}

	if !IsTerminal() {
		return ErrNotATTY
	}

	if err := c.ensureTmuxPath(); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, c.tmuxPath, "attach-session", "-t", sessionTarget(name), "-d")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Kill terminates a session or the entire tmux server if all is true.
func (c *OSClient) Kill(ctx context.Context, name string, all bool) error {
	if all {
		_, err := c.run(ctx, "kill-server")
		if err != nil {
			errMsg := err.Error()
			if strings.Contains(errMsg, "no server running") ||
				strings.Contains(errMsg, "error connecting to") {
				return nil
			}
			return err
		}
		return nil
	}

	if name == "" {
		return fmt.Errorf("%w: session name required", ErrValidation)
	}

	exists, err := c.HasSession(ctx, name)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%w: session '%s'", ErrNotFound, name)
	}

	_, err = c.run(ctx, "kill-session", "-t", sessionTarget(name))
	return err
}

// CapturePane captures lines from the active pane.
func (c *OSClient) CapturePane(ctx context.Context, name string, lines int) (string, error) {
	exists, err := c.HasSession(ctx, name)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("%w: session '%s'", ErrNotFound, name)
	}

	if lines <= 0 {
		lines = 25
	}
	startLine := fmt.Sprintf("-%d", lines)
	out, err := c.run(ctx, "capture-pane", "-p", "-t", paneTarget(name), "-S", startLine)
	if err != nil {
		return "", fmt.Errorf("capture-pane failed: %w", err)
	}
	return string(out), nil
}

// SendKeys injects keys into the target session.
func (c *OSClient) SendKeys(ctx context.Context, name, payload string, enter bool, raw bool) error {
	exists, err := c.HasSession(ctx, name)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%w: session '%s'", ErrNotFound, name)
	}

	target := paneTarget(name)
	var args []string
	if raw {
		args = []string{"send-keys", "-t", target, payload}
	} else {
		args = []string{"send-keys", "-t", target, "-l", payload}
	}

	_, err = c.run(ctx, args...)
	if err != nil {
		return fmt.Errorf("send-keys failed: %w", err)
	}

	if enter {
		_, err = c.run(ctx, "send-keys", "-t", target, "Enter")
		if err != nil {
			return fmt.Errorf("send-keys Enter failed: %w", err)
		}
	}
	return nil
}

// Doctor checks tmux binary, version, and server status.
func (c *OSClient) Doctor(ctx context.Context) (*DoctorResult, error) {
	res := &DoctorResult{}
	path, err := exec.LookPath("tmux")
	if err != nil {
		hint := DetectInstallInfo()
		res.InstallHint = &hint
		return res, nil
	}
	res.TmuxInstalled = true
	res.TmuxPath = path

	verOut, err := c.run(ctx, "-V")
	if err == nil {
		res.TmuxVersion = strings.TrimSpace(string(verOut))
	}

	sessions, err := c.ListSessions(ctx)
	if err == nil {
		res.ServerRunning = len(sessions) > 0 || c.isServerAlive(ctx)
	}

	return res, nil
}

func (c *OSClient) isServerAlive(ctx context.Context) bool {
	_, err := c.run(ctx, "list-clients")
	return err == nil
}
