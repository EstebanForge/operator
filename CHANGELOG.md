# Changelog

All notable changes to `operator` are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-09-07

<!-- RELEASE:START 0.2.0 -->

Adds a Tmux Server Management submenu to the interactive TUI. Restart and
kill-server are confirm-gated; the config reload applies changes without
touching sessions (tmux reads its config only at server start, so a reload
is the only way around a restart).

### Added

- `Tmux Server Management` entry in the TUI root menu, with a submenu:
  Restart Tmux Server, Kill Tmux Server, Reload Config, Server Status, and
  Back.
- `Restart Tmux Server`: kills the server and starts a fresh one with a
  detached `main` session, so the restarted server is attachable right
  away. A restart also reloads the tmux config.
- `Kill Tmux Server`: the TUI counterpart of `kill -a`. A server that is
  already down is a no-op.
- `Reload Config`: runs `source-file` on the first existing user config
  (`~/.tmux.conf`, then the XDG location). Non-destructive; a server that
  is not running is a no-op, and a missing config reports the searched
  paths.
- `Server Status`: tmux version, binary path, and server state (reuses
  the `doctor` check).

<!-- RELEASE:END 0.2.0 -->

## [0.1.1] - 2026-09-07

<!-- RELEASE:START 0.1.1 -->

Fixes the Homebrew install flow found in 0.1.0.

### Fixed

- `operator setup` no longer drops the `opr` alias inside Homebrew's
  Cellar (off `$PATH`, dead after `brew upgrade`). The alias now lands
  beside the invoked binary, and the manual-alias fallback references
  the invoked path.
- The Homebrew formula installs the `opr` alias itself through
  `bin.install_symlink` (tap-side change, already live).

### Upgrade notes (0.1.0 → 0.1.1)

Upgrading heals both known issues automatically: removing the old keg
deletes any stray Cellar alias, and the upgrade's install step creates
the missing `$PATH` alias. If `brew upgrade` reports an `opr` conflict,
a file named `opr` exists in Homebrew's bin directory and came from
something else; remove that file and rerun the upgrade.

<!-- RELEASE:END 0.1.1 -->

## [0.1.0] - 2026-09-07

<!-- RELEASE:START 0.1.0 -->

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

<!-- RELEASE:END 0.1.0 -->
