# AsusCtl TUI

A terminal UI for [asusctl](https://gitlab.com/asus-linux/asusctl) — the Linux control daemon for ASUS ROG/TUF laptops.

**Pure Go, zero external dependencies.** Uses only the standard library — no tcell, bubbletea, or any third-party packages. ANSI escape sequences and raw terminal I/O via syscalls.

```
┌──────────────────────────────────────────────────────────────────┐
│ R  AsusCtl Control Center                       ● connected     │
│ 1:Profile  2:Keyboard  3:Aura RGB  4:Battery  5:Fans  6:BIOS   │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Power Profile                                                   │
│                                                                  │
│  ▸ ⚡ Performance         Max clocks, aggressive fans            │
│    ⚖  Balanced            Auto-tuned defaults          ACTIVE   │
│    🔇 Quiet               Minimal fan noise                      │
│                                                                  │
├──────────────────────────────────────────────────────────────────┤
│ 1-7:Tab  ↑↓:Navigate  ←→:Adjust  Enter:Apply  q:Quit          │
└──────────────────────────────────────────────────────────────────┘
```

## Features

| Tab | Controls |
|-----|----------|
| **1: Profile** | Switch Performance / Balanced / Quiet |
| **2: Keyboard** | Backlight brightness (off / low / med / high) |
| **3: Aura RGB** | 12 lighting modes (Static, Breathe, Rainbow...) |
| **4: Battery** | Charge limit slider (20-100%), one-shot full charge |
| **5: Fans** | Interactive ASCII fan curve editor with presets, CPU/GPU |
| **6: BIOS** | Panel Overdrive, GPU MUX toggle |
| **7: Console** | Run any raw asusctl command, output log |

## Requirements

- **Go 1.21+** (build only)
- **asusctl + asusd** installed and running
- A terminal with true-color support (most modern terminals)

## Build & Run

```bash
chmod +x build.sh
./build.sh
./asusctl-gui
```

Or manually:

```bash
go build -o asusctl-gui .
./asusctl-gui
```

Install system-wide:

```bash
sudo install -m 755 asusctl-gui /usr/local/bin/
```

## Controls

| Key | Action |
|-----|--------|
| `1`-`7` | Switch tab |
| `↑` `↓` | Navigate / adjust fan speed |
| `←` `→` | Navigate / adjust values |
| `Enter` | Apply selection |
| `Tab` | Switch CPU/GPU fan (Fans tab) |
| `s` `b` `p` `f` | Fan presets: Silent, Balanced, Performance, Full |
| `e` | Toggle custom fan curves on/off |
| `q` / `Ctrl-C` | Quit |

## Architecture

```
main.go       Entry point, event loop, signal handling
terminal.go   Raw mode, ANSI output, key input (stdlib only)
theme.go      Colors, box drawing, UI primitives
app.go        App state, all 7 tab renderers and input handlers
backend.go    asusctl CLI wrapper (os/exec)
```

The terminal is put into raw mode via `TCGETS`/`TCSETS` ioctls. All rendering uses buffered ANSI escape sequences (24-bit color) flushed as a single write per frame. Keyboard input is read byte-by-byte with escape sequence parsing for arrow keys and modifiers.

## License

MIT
