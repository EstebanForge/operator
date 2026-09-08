package tmux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeTmux replaces the tmux binary with a script that appends one line per
// invocation (args space-joined) to a call log, then runs the given shell
// body. This keeps server-op tests off any live developer tmux server.
func fakeTmux(t *testing.T, body string) (*OSClient, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "tmux")
	logPath := filepath.Join(dir, "calls.log")
	script := "#!/bin/sh\n" +
		"for a in \"$@\"; do printf '%s ' \"$a\" >> '" + logPath + "'; done\n" +
		"printf '\\n' >> '" + logPath + "'\n" +
		body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return &OSClient{tmuxPath: path}, logPath
}

// callLog returns the recorded tmux invocations, one string per call.
func callLog(t *testing.T, logPath string) []string {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	raw := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	calls := make([]string, len(raw))
	for i, line := range raw {
		calls[i] = strings.TrimRight(line, " ")
	}
	return calls
}

func TestKillServer(t *testing.T) {
	client, logPath := fakeTmux(t, `exit 0`)

	if err := client.KillServer(t.Context()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	calls := callLog(t, logPath)
	if len(calls) != 1 || !strings.HasPrefix(calls[0], "kill-server") {
		t.Fatalf("expected a single kill-server call, got %v", calls)
	}
}

func TestKillServer_ServerAlreadyDown(t *testing.T) {
	client, _ := fakeTmux(t, `echo "no server running on /tmp/tmux-0/default" >&2; exit 1`)

	if err := client.KillServer(t.Context()); err != nil {
		t.Fatalf("expected a down server to be a no-op success, got %v", err)
	}
}

func TestRestartServer_KillsThenStartsMain(t *testing.T) {
	client, logPath := fakeTmux(t, `case "$1" in
		kill-server) exit 0 ;;
		has-session) echo "can't find session: main" >&2; exit 1 ;;
		new-session) exit 0 ;;
		list-sessions) printf 'main\t1\t1700000000\t0\t/tmp\n' ;;
		*) exit 0 ;;
	esac`)

	sess, err := client.RestartServer(t.Context())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if sess.Name != restartSessionName {
		t.Fatalf("expected fresh session %q, got %q", restartSessionName, sess.Name)
	}

	calls := callLog(t, logPath)
	if len(calls) < 2 {
		t.Fatalf("expected kill-server then new-session calls, got %v", calls)
	}
	if !strings.HasPrefix(calls[0], "kill-server") {
		t.Fatalf("expected kill-server first, got %q", calls[0])
	}
	newSessionIdx := -1
	for i, call := range calls {
		if strings.HasPrefix(call, "new-session") {
			newSessionIdx = i
			break
		}
	}
	if newSessionIdx < 0 {
		t.Fatalf("expected a new-session call, got %v", calls)
	}
	if !strings.Contains(calls[newSessionIdx], "-s main") || !strings.Contains(calls[newSessionIdx], "-d") {
		t.Fatalf("expected detached 'main' session creation, got %q", calls[newSessionIdx])
	}
}

func TestReloadConfig_SourcesUserConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configPath := filepath.Join(home, ".tmux.conf")
	if err := os.WriteFile(configPath, []byte("set -g base-index 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	client, logPath := fakeTmux(t, `if [ "$1" = source-file ]; then exit 0; fi; exit 1`)

	if err := client.ReloadConfig(t.Context()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	calls := callLog(t, logPath)
	if len(calls) != 1 || calls[0] != "source-file "+configPath {
		t.Fatalf("expected source-file %q call, got %v", configPath, calls)
	}
}

func TestReloadConfig_NoServerIsNoOp(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, ".tmux.conf"), []byte("set -g base-index 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	client, _ := fakeTmux(t, `echo "no server running on /tmp/tmux-0/default" >&2; exit 1`)

	if err := client.ReloadConfig(t.Context()); err != nil {
		t.Fatalf("expected reload without a server to succeed as no-op, got %v", err)
	}
}

func TestReloadConfig_SourceErrorPropagates(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, ".tmux.conf"), []byte("bogus-command\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	client, _ := fakeTmux(t, `echo "bogus-command" >&2; exit 1`)

	if err := client.ReloadConfig(t.Context()); err == nil {
		t.Fatal("expected a config error to propagate, got nil")
	}
}

func TestReloadConfig_NoConfigFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")

	client, logPath := fakeTmux(t, `exit 0`)

	err := client.ReloadConfig(t.Context())
	if err == nil || !strings.Contains(err.Error(), "no tmux config file found") {
		t.Fatalf("expected missing-config error, got %v", err)
	}
	if calls := callLog(t, logPath); calls != nil {
		t.Fatalf("expected no tmux invocation without a config file, got %v", calls)
	}
}

func TestResolveTmuxConfigPath(t *testing.T) {
	t.Run("prefers ~/.tmux.conf over XDG", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		xdg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", xdg)

		legacy := filepath.Join(home, ".tmux.conf")
		xdgConf := filepath.Join(xdg, "tmux", "tmux.conf")
		if err := os.MkdirAll(filepath.Dir(xdgConf), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(legacy, []byte("# legacy\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(xdgConf, []byte("# xdg\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		got, err := resolveTmuxConfigPath()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != legacy {
			t.Fatalf("expected %q, got %q", legacy, got)
		}
	})

	t.Run("falls back to XDG_CONFIG_HOME location", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		xdg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", xdg)

		xdgConf := filepath.Join(xdg, "tmux", "tmux.conf")
		if err := os.MkdirAll(filepath.Dir(xdgConf), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(xdgConf, []byte("# xdg\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		got, err := resolveTmuxConfigPath()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != xdgConf {
			t.Fatalf("expected %q, got %q", xdgConf, got)
		}
	})

	t.Run("falls back to default XDG path", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("XDG_CONFIG_HOME", "")

		xdgConf := filepath.Join(home, ".config", "tmux", "tmux.conf")
		if err := os.MkdirAll(filepath.Dir(xdgConf), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(xdgConf, []byte("# xdg\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		got, err := resolveTmuxConfigPath()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != xdgConf {
			t.Fatalf("expected %q, got %q", xdgConf, got)
		}
	})
}
