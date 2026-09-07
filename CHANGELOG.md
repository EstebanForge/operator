# Changelog

All notable changes to `operator` are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-09-07

First public beta. Single-binary session multiplexer and orchestrator on top
of native `tmux`, with a human TUI and an agent-facing machine contract.

### Added

- Session commands: `new` (auto-generated bilingual 3-word names), `ls`,
  `join` (attach or switch-client), `peek` (pane capture), `send` (literal
  or raw key injection), `kill` (confirm modal, `-a` for kill-server),
  `doctor` (tmux diagnostics with install hints), `version`.
- `tab` command: creates a tmux window (tab) in an existing session at the
  next free index, with working-directory resolution (flag, session pane
  path, operator cwd) validated on disk.
- `setup` command: creates the `opr` symlink beside the binary; refuses to
  touch a foreign `opr` entry (exit 3).
- Interactive TUI: session-first menu (root lists sessions with window
  counts, per-session submenu for Attach / Tab / Peek / Kill / Back),
  Esc navigation, confirm modals before kills, and hint lines teaching
  tmux detach/exit keys.
- Machine contract: `--json` output with stable schemas, deterministic exit
  codes (0 success, 1 daemon, 2 not found, 3 conflict, 4 validation),
  diagnostics on stderr only, zero ANSI in JSON mode.
- Exact-match tmux targeting (`=<name>`, `=<name>:`, `=<session>:`) so a
  partial name can never address the wrong session.
- Post-attach hints: detach confirmation with reattach/kill commands, or a
  closed notice, printed to stderr for humans only.
- `make release`: version guard, cross-compiled tarballs (darwin/linux,
  amd64/arm64) with SHA-256 checksums.

### Security

- No shell interpolation: tmux invocations go through `os/exec` argument
  lists; session names are sanitized (whitespace to `-`, non
  alphanumeric/dash/underscore stripped).
- Setup never writes to shell rc files; it prints manual instructions when
  it cannot symlink.
