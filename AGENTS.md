# PROJECT KNOWLEDGE BASE

**Generated:** 2026-09-05

## OVERVIEW
Project: **operator**
Stack: Go 1.27, Cobra (CLI routing), Charm Huh (TUI forms), go-isatty (terminal detection), tmux (external multiplexer binary)

## STRUCTURE
* `cmd/`: CLI commands, Cobra configuration, exit handlers, JSON output serialization, and interactive TUI loop.
* `cmd/root.go`: Base command, global `--json` flag, and non-TTY execution validation.
* `cmd/errors.go`: Exit code mapping and JSON error output serializers.
* `cmd/tui.go`: Charm `huh` interactive menu loop and confirmation modals.
* `pkg/tmux/`: Subprocess communication client for native `tmux`.
* `pkg/tmux/client.go`: `Client` interface, `OSClient` implementation, tab-delimited parser, and session name sanitizer.
* `docs/`: Technical specifications and Architecture Decision Records (ADRs).
* `docs/SPECS.md`: Primary system specification, machine schemas, and test checklist.

## COMMANDS
| Action | Command |
| --- | --- |
| Full Checks | `make check` (fmt, vet, lint, test, build) |
| Local Install | `make build-signed` (installs to `~/.local/bin/operator` & `opr`) |
| Test | `make test` |
| Lint | `make lint` |
| Build | `make build` |
| Run | `./bin/operator [command]` |
| Clean | `make clean` |

## CODING STANDARDS
* **Language**: Go standard library conventions. Format code with `gofmt`.
* **Style**: Interface-driven design (`tmux.Client`) allowing full mock substitution in unit tests.
* **Architecture**: Clean separation between CLI presentation (`cmd/`) and tmux execution (`pkg/tmux/`).
* **Machine Contract**: Zero ANSI escape sequences in stdout when `--json` is set. Diagnostic logs and errors write to stderr.
* **Exit Codes**: Strict exit codes enforced across all commands (0: Success, 1: Daemon/OS error, 2: Session not found, 3: Conflict, 4: Validation / Invocation / Non-TTY error).

## WHERE TO LOOK
* **Source**: `main.go`, `cmd/`, `pkg/tmux/`
* **Tests**: `cmd/cmd_test.go`, `pkg/tmux/client_test.go`
* **Docs**: `docs/SPECS.md`, `README.md`

## NOTES
* **Delimiter Safety**: `list-sessions` parses with tab characters (`\t`) to prevent collisions with pipes in directory paths.
* **Timestamps**: Convert unix epoch timestamps (`#{session_created}`) to RFC 3339 format.
* **TTY Guards**: `operator` (bare invocation) and `operator join` require an interactive terminal. When invoked in non-TTY environments without subcommands, exit immediately with code `4`.
* **Subprocess Security**: Do not mutate shell dotfiles (`~/.bashrc`, `~/.zshrc`) during `setup`. Output manual instructions to stderr if symlink creation fails.
