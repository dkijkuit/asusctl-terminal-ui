# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
# Build (produces stripped binary in current dir)
go build -ldflags="-s -w" -o asusctl-gui .

# Or use the build script
./build.sh

# Install system-wide
sudo install -m 755 asusctl-gui /usr/local/bin/

# Run (requires asusctl + asusd installed and running, true-color terminal)
./asusctl-gui
```

There are no tests. Go 1.21+ is required. There are zero external dependencies (stdlib only).

## Architecture

Single-package (`main`) TUI application (~1800 lines across 5 files) that wraps the `asusctl` CLI to control ASUS ROG/TUF laptop hardware.

**main.go** — Entry point. Sets up terminal raw mode, signal handlers (SIGINT/SIGTERM for cleanup, SIGWINCH for resize), and runs the event loop (read key → handle input → render).

**terminal.go** — Low-level terminal I/O. Manages raw mode via syscall ioctls (TCGETS/TCSETS/TIOCGWINSZ), parses ANSI key sequences byte-by-byte into `KeyEvent` structs, and provides a buffered writer that builds a full frame then flushes atomically to stdout.

**app.go** — Application state and all UI logic. The `App` struct holds all state (active tab, focus index, per-feature values like profile/kbdLevel/chargeLimit/fanSpeeds). Contains 7 tab renderers and their input handlers. Each tab is a render function + input handler dispatched by `activeTab`. State changes trigger re-renders on the next loop iteration.

**backend.go** — Wraps `asusctl` CLI commands via `os/exec` with a 5-second timeout. Methods map 1:1 to asusctl subcommands (profile, led, aura, batt, fan, bios). Returns stdout/stderr strings and errors.

**theme.go** — Color palette (RGB `Color` type), box-drawing primitives (DrawBox, FillRect, HLine), and UI component helpers (DrawBar, DrawButton, DrawToggle).

## Key Patterns

- **Rendering**: All drawing goes through `Terminal`'s buffer (`term.Text()`, `term.DrawBox()`, etc.) then `term.Flush()` writes once per frame. Uses ANSI 24-bit color escapes and alternate screen buffer.
- **Input**: `terminal.ReadKey()` reads raw bytes, translates escape sequences (arrows, page up/down, ctrl combos) into a `KeyEvent`. The app dispatches to the active tab's handler.
- **Backend calls**: Every hardware interaction shells out to `asusctl` with a timeout goroutine. Output is parsed from stdout strings. There is no D-Bus or direct daemon communication.
- **Fan curves**: Stored as `fanSpeeds[2][8]` (CPU/GPU × 8 temperature points) with fixed temperature breakpoints in `fanTemps[8]`. The fan tab renders an ASCII graph with interactive point editing.
- **Console tab**: Accepts raw asusctl commands typed by the user, maintains a 100-line scrollable log buffer.
