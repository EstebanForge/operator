# Project Specification & Architecture Plan: `operator`

## 1. Executive Summary & Goals

`operator` is a zero-cruft, single-binary CLI tool written in Go that acts as an intuitive session multiplexer and orchestrator on top of native `tmux`.

It addresses two distinct consumers with equal priority:

1. **Human Operators (especially juniors/sysadmins):** Interactive TUI (via Charm `huh`), zero cryptic flags, safe defaults, guided menus, and self-installing shorthands (`opr`).
2. **Autonomous LLM Agents:** Fully programmatic subcommands, strict machine-readable JSON schemas (`--json`), deterministic exit codes, and non-blocking TTY-safe execution.

## 2. Architecture Decision Records (ADRs)

### ADR 001: Subprocess Wrapping over Native C Bindings

- **Status:** Accepted
- **Context:** Go can interact with `tmux` via CGO/libtmux wrappers or direct `os/exec` calls to the `tmux` CLI binary.
- **Decision:** Use standard library `os/exec` directly against `tmux`.
- **Consequences:**
  - Eliminates CGO and cross-compilation friction.
  - Preserves single-binary portability across Linux (Fedora, Debian, Arch) and macOS.
  - Dependency on system `tmux` being installed. Handled via pre-flight check (`doctor`/exec check).

### ADR 002: Dual-Mode Execution (TTY Detection)

- **Status:** Accepted
- **Context:** Running the binary without arguments should show a friendly menu to humans, but must never hang an automated script or LLM agent waiting for interactive stdin.
- **Decision:** Check `isatty.IsTerminal()` on stdin/stdout when zero arguments are provided.
  - If TTY: Launch interactive TUI.
  - If Non-TTY (pipe/subshell): Output CLI usage/help to stderr and exit with code `4`.
  - If Subcommand passed: Always bypass TUI regardless of TTY status.

### ADR 003: Deterministic Machine Contract (`--json`)

- **Status:** Accepted
- **Context:** Agents break when CLI tools alter output formatting, introduce ANSI escape sequences, or write debugging logs to stdout.
- **Decision:**
  - Global `--json` flag formats all query and mutation outputs to JSON.
  - Output on `stdout` is exclusively valid JSON (no mixed status strings).
  - Informational warnings and logs must route to `stderr`.
  - Strict JSON schemas with stable keys.

### ADR 004: Dual-Name Identity via Binary Symlink (`opr`)

- **Status:** Accepted
- **Context:** The name `operator` has personality but is 8 characters. Modifying shell rc files (`.zshrc`) silently is an anti-pattern.
- **Decision:** Implement a self-contained command (`operator setup`) that creates a relative/absolute symlink named `opr` directly beside the `operator` binary in `$PATH` (e.g., `~/.local/bin/opr` -> `~/.local/bin/operator`). If filesystem permissions deny symlink creation, print the manual alias command to stderr. Do not write to user dotfiles automatically.

### ADR 005: Safe Delimitation and Terminal Safety

- **Status:** Accepted
- **Context:** Parsing `tmux list-sessions` with pipe (`|`) delimiters causes collisions when session names or paths contain pipe characters. Attaching to tmux inside an existing session or from a non-TTY environment fails or corrupts terminal state.
- **Decision:**
  - Use tab (`\t`) as the field delimiter for `tmux list-sessions`.
  - Treat a "no server running" tmux exit status as an empty session list (`[]`), not an error.
  - Format `created_at` timestamps as RFC 3339 (ISO 8601) strings parsed from epoch seconds (`#{session_created}`).
  - Guard `operator join` against non-TTY invocation with exit code `4`.
  - Detect `$TMUX` on `operator join`: use `switch-client -t <name>` when inside tmux, and `attach-session -t <name> -d` when outside.
  - Send literal keys with `tmux send-keys -l` by default to avoid control code misinterpretation.

## 3. Command-Line Interface Specification

### Global Flags

- `--json` (bool, default: `false`): Formats command output as JSON.
- `-h, --help`: Displays help.

### Subcommands & Behaviors

| **Command** | **Arguments**      | **Flags**                                              | **Non-Interactive / Agent Behavior**                         | **Interactive / Human Behavior**             |
| ----------- | ------------------ | ------------------------------------------------------ | ------------------------------------------------------------ | -------------------------------------------- |
| `[none]`    | None               | Global only                                            | Returns error code `4` + usage if non-TTY.                   | Boots full interactive TUI menu loop.        |
| `ls`        | None               | `--json`                                               | Returns session list (table or JSON array).                  | Prints formatted status table.               |
| `new`       | `[name]`           | `-d, --dir <path>`  `-c, --cmd <string>`  `--detached` | Creates session. Auto-generates 3-word bilingual name if omitted. Auto-sanitizes whitespace to `-`. Fails if session exists. | Prompted with default auto-generated name if `<name>` omitted or blank. |
| `join`      | `<name>`           | None                                                   | Fails with code `4` if non-TTY or `<name>` omitted. Attaches with `-d` (or switches client if inside `$TMUX`). | Single-select list of sessions if omitted.   |
| `peek`      | `<name>`           | `-l, --lines <int>` (default 25)  `--json`             | Reads last $N$ lines from active pane via `capture-pane`.    | Displays paginated preview with back option. |
| `send`      | `<name> <payload>` | `--no-enter` (default false)  `--raw` (default false)  | Injects keys into session via `send-keys`. Defaults to literal text (`-l`); sends raw keys if `--raw`. | Not in TUI menu (agent/script focused).      |
| `kill`      | `<name>`           | `-a, --all`  `-f, --force`                             | Kills session. Fails if name missing unless `-a` is passed. `-f` skips prompt. | Confirmation modal required unless `-f`.     |
| `setup`     | None               | None                                                   | Creates `opr` symlink in same directory as executable.       | Prints resolution status and verification.   |
| `doctor`    | None               | `--json`                                               | Checks `tmux` binary presence, version, and socket access.   | Diagnostic output for troubleshooting.       |

## 4. Machine Schemas (JSON Specification)

### 4.1. Session Object (`operator ls --json`)

Array of session objects:

JSON

```
[
  {
    "name": "backup-worker",
    "windows": 1,
    "created_at": "2026-09-05T19:30:00Z",
    "is_attached": false,
    "path": "/srv/storage"
  }
]
```

### 4.2. Mutation Result (`operator new`, `operator kill`, `operator send`)

JSON

```
{
  "status": "ok",
  "action": "create|kill|send",
  "session": "backup-worker",
  "details": {
    "detached": true,
    "directory": "/srv/storage"
  }
}
```

### 4.3. Output Inspection (`operator peek <name> --json`)

JSON

```
{
  "session": "backup-worker",
  "lines_captured": 25,
  "output": "line 1\nline 2\nline 3\n"
}
```

### 4.4. Error Object (Sent to `stderr` when `--json` is active)

JSON

```
{
  "status": "error",
  "code": 2,
  "message": "session 'worker' not found"
}
```

## 5. Exit Code Standards

- **`0` (Success):** Operation succeeded.
- **`1` (Internal / Daemon Error):** `tmux` execution failed, binary missing, or fatal OS error.
- **`2` (Not Found / Target Missing):** Target session does not exist.
- **`3` (Conflict):** Session already exists on `new`.
- **`4` (Validation / Invocation Error):** Missing required argument in non-interactive mode, invocation of interactive command in non-TTY environment, or invalid characters in session name.

## 6. Implementation Plan for Downstream LLM Agent

### Phase 1: Core Foundation & Data Layer

1. Scaffold Go module (`go.mod`) with Go standard library + `spf13/cobra`, `charmbracelet/huh`, and `mattn/go-isatty`.
2. Implement `pkg/tmux/client.go`:
   - Use tab-delimited formatted strings: `tmux list-sessions -F "#{session_name}\t#{session_windows}\t#{session_created}\t#{session_attached}\t#{pane_current_path}"`.
   - Parse `#{session_created}` (Unix epoch seconds) to RFC 3339 formatted timestamp.
   - Differentiate "no server running" output from system errors (return empty slice `[]Session` with nil error).
   - Implement wrappers: `ListSessions()`, `NewSession()`, `Attach()`, `Kill()`, `CapturePane()`, `SendKeys()`.
   - Implement `$TMUX` detection in `Attach()`: use `switch-client` if inside tmux, `attach-session -d` if outside. Enforce TTY check.
   - Implement `SendKeys()`: default to literal text (`-l`), support raw keycode injection via option.
   - Add session name sanitization logic (convert spaces to `-`, strip non-alphanumeric/dash/underscore).

### Phase 2: CLI Engine (Cobra)

1. Implement `cmd/root.go` with global `--json` flag and TTY detection.
2. Implement subcommands (`ls`, `new`, `join`, `peek`, `kill`, `send`, `doctor`).
3. Enforce the non-interactive contract: fail fast with exit code `4` if arguments are missing or invalid, or if stdin is not a TTY for interactive commands.
4. Ensure JSON serializer formats output with zero ANSI color tags.

### Phase 3: Interactive TUI (Huh Engine)

1. Implement `cmd/tui.go` containing the main loop:
   - Header showing active session counts.
   - Menu: `Attach`, `New`, `Peek`, `Kill`, `Exit`.
   - Guard empty states (e.g., if sessions == 0, disable `Attach`/`Peek`/`Kill` or route to `New`).
2. Integrate safety confirmation modals before executing destructive actions (`kill`).

### Phase 4: Self-Installer & Aliasing

1. Implement `cmd/setup.go`:
   - Detect binary path using `os.Executable()`.
   - Resolve symlinks to find the real directory.
   - Create symbolic link `opr -> operator` in that folder.
   - If permission denied, print manual alias configuration instructions to stderr without modifying shell rc files.

### Phase 5: Verification & Testing Checklist

- Test pipeline redirection: `operator ls --json | jq .` works cleanly.
- Test empty state: `operator ls --json` returns `[]` with exit code 0 when tmux server is not running.
- Test pipe-in failure: `echo "foo" | operator` prints usage to stderr, returns exit code 4, and does not hang.
- Test `opr` symlink creation and execution parity with `operator`.
- Test client switching when `$TMUX` is present vs detaching remote client when `$TMUX` is absent on `operator join`.