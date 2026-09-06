# operator

`operator` is a single-binary CLI tool written in Go. It operates as a session multiplexer on top of native `tmux`.

It serves two targets:
1. **Human Operators:** Interactive terminal UI (Charm `huh`), simple prompts, and safe defaults.
2. **Autonomous Agents:** Strict JSON output (`--json`), deterministic exit codes, and non-blocking execution.

## Requirements

- Linux or macOS
- `tmux` (installed and available in `$PATH`)
- Go 1.22 or newer (to compile from source)

## Installation & Build

### Development Checks

Run the verification pipeline (formats code, runs `go vet`, runs `golangci-lint`, executes tests with `-race`, and builds the binary):

```bash
make check
```

### Local Build & Installation

Build, ad-hoc sign (on macOS), and install `operator` and `opr` alias to `~/.local/bin`:

```bash
make build-signed
```

Verify with:

```bash
operator version
opr version
```

Alternatively, build locally without installing to `~/.local/bin`:

```bash
make build
# Binary produced at ./bin/operator
```

## Usage

### Interactive Mode (Human)

When invoked directly from an interactive terminal with no arguments:

```bash
operator
```

This launches the interactive menu loop. You can create, attach, preview, or kill sessions.

### Command-Line Interface (Agent / Script)

Global flags:
- `--json`: Formats output as valid JSON on `stdout`. Errors are emitted as JSON on `stderr`.
- `-h, --help`: Displays usage help.

| Command | Arguments | Flags | Description |
| --- | --- | --- | --- |
| `ls` | None | `--json` | Lists active sessions. Returns `[]` if no server is running. |
| `new` | `[name]` | `-d, --dir <path>`<br>`-c, --cmd <string>`<br>`--detached` | Creates a new session. Auto-generates a 3-word bilingual (English + Spanish) name if omitted or left blank. Sanitizes spaces to `-`. Fails if session exists. |
| `join` | `<name>` | None | Attaches to session with `-d`. Switches client if inside `$TMUX`. Fails with code `4` if non-TTY. |
| `peek` | `<name>` | `-l, --lines <int>` (default 25)<br>`--json` | Captures the last $N$ lines from the active pane. |
| `send` | `<name> <payload>` | `--no-enter`<br>`--raw` | Injects keys into session. Literal strings by default (`-l`); raw keycodes if `--raw`. |
| `kill` | `<name>` | `-a, --all`<br>`-f, --force` | Kills session. Confirmation modal required unless `-f` is passed. |
| `setup` | None | `--json` | Creates `opr` symlink next to binary. Outputs alias command if permissions fail. |
| `doctor` | None | `--json` | Validates `tmux` installation, path, version, and server socket status. |

## Machine Contract (JSON & Exit Codes)

### Exit Codes

- `0` (Success): Command executed successfully.
- `1` (Daemon / System Error): `tmux` binary missing or underlying system call failed.
- `2` (Not Found): Target session does not exist.
- `3` (Conflict): Session already exists when running `new`.
- `4` (Validation / Invocation Error): Missing required argument, invalid characters, or interactive invocation in non-TTY.

### JSON Schema Examples

List sessions (`operator ls --json`):

```json
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

Mutation response (`operator new backup-worker --detached --json`):

```json
{
  "status": "ok",
  "action": "create",
  "session": "backup-worker",
  "details": {
    "detached": true,
    "directory": "/srv/storage"
  }
}
```

Inspection response (`operator peek backup-worker -l 25 --json`):

```json
{
  "session": "backup-worker",
  "lines_captured": 25,
  "output": "line 1\nline 2\n"
}
```

Error response (sent to `stderr` with non-zero exit code):

```json
{
  "status": "error",
  "code": 2,
  "message": "session not found: session 'worker'"
}
```

## Testing

Execute the test suite:

```bash
go test -v ./...
```
