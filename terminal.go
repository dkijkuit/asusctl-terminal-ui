package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Terminal — raw mode, ANSI escape sequences, input handling
// Uses only syscall/os — no external deps
// ═══════════════════════════════════════════════════════════════════════════════

type Terminal struct {
	origTermios syscall.Termios
	width       int
	height      int
	buf         strings.Builder
	mu          sync.Mutex
	inRaw       bool
}

// termios ioctl constants
const (
	ioctlGetTermios = 0x5401 // TCGETS
	ioctlSetTermios = 0x5402 // TCSETS
	ioctlGetWinSz   = 0x5413 // TIOCGWINSZ
)

type winsize struct {
	Row, Col, Xpixel, Ypixel uint16
}

func NewTerminal() *Terminal {
	t := &Terminal{}
	t.updateSize()
	return t
}

func (t *Terminal) updateSize() {
	ws := &winsize{}
	_, _, _ = syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(syscall.Stdout),
		uintptr(ioctlGetWinSz),
		uintptr(unsafe.Pointer(ws)))
	t.width = int(ws.Col)
	t.height = int(ws.Row)
	if t.width < 40 {
		t.width = 80
	}
	if t.height < 10 {
		t.height = 24
	}
}

func (t *Terminal) Width() int  { return t.width }
func (t *Terminal) Height() int { return t.height }

func (t *Terminal) EnterRaw() error {
	var orig syscall.Termios
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(syscall.Stdin),
		uintptr(ioctlGetTermios),
		uintptr(unsafe.Pointer(&orig)))
	if errno != 0 {
		return fmt.Errorf("get termios: %v", errno)
	}
	t.origTermios = orig

	raw := orig
	// Input: no SIGINT/SIGQUIT, no break, no CR→NL, no parity, no strip, no XON/XOFF
	raw.Iflag &^= syscall.BRKINT | syscall.ICRNL | syscall.INPCK | syscall.ISTRIP | syscall.IXON
	// Output: no post-processing
	raw.Oflag &^= syscall.OPOST
	// Control: 8-bit chars
	raw.Cflag |= syscall.CS8
	// Local: no echo, no canonical, no signals, no extended
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.IEXTEN | syscall.ISIG
	// Read returns after 1 byte or 100ms timeout
	raw.Cc[syscall.VMIN] = 0
	raw.Cc[syscall.VTIME] = 1

	_, _, errno = syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(syscall.Stdin),
		uintptr(ioctlSetTermios),
		uintptr(unsafe.Pointer(&raw)))
	if errno != 0 {
		return fmt.Errorf("set raw: %v", errno)
	}
	t.inRaw = true

	// Hide cursor, enable alternate screen buffer, enable mouse (for potential future use)
	fmt.Fprint(os.Stdout, "\033[?1049h\033[?25l")
	return nil
}

func (t *Terminal) ExitRaw() {
	if !t.inRaw {
		return
	}
	// Show cursor, restore main screen buffer
	fmt.Fprint(os.Stdout, "\033[?25h\033[?1049l")
	syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(syscall.Stdin),
		uintptr(ioctlSetTermios),
		uintptr(unsafe.Pointer(&t.origTermios)))
	t.inRaw = false
}

// ─── Buffered ANSI output ────────────────────────────────────────────────────

func (t *Terminal) Clear() {
	t.buf.Reset()
}

func (t *Terminal) MoveTo(x, y int) {
	fmt.Fprintf(&t.buf, "\033[%d;%dH", y+1, x+1)
}

func (t *Terminal) SetFg(r, g, b int) {
	fmt.Fprintf(&t.buf, "\033[38;2;%d;%d;%dm", r, g, b)
}

func (t *Terminal) SetBg(r, g, b int) {
	fmt.Fprintf(&t.buf, "\033[48;2;%d;%d;%dm", r, g, b)
}

func (t *Terminal) ResetStyle() {
	t.buf.WriteString("\033[0m")
}

func (t *Terminal) Bold() {
	t.buf.WriteString("\033[1m")
}

func (t *Terminal) Dim() {
	t.buf.WriteString("\033[2m")
}

func (t *Terminal) Underline() {
	t.buf.WriteString("\033[4m")
}

func (t *Terminal) Reverse() {
	t.buf.WriteString("\033[7m")
}

func (t *Terminal) Write(s string) {
	t.buf.WriteString(s)
}

func (t *Terminal) Flush() {
	t.mu.Lock()
	defer t.mu.Unlock()
	// Home cursor and hide it during redraw to avoid flicker
	os.Stdout.WriteString("\033[?25l\033[H")
	os.Stdout.WriteString(t.buf.String())
	os.Stdout.WriteString("\033[?25h")
}

// ─── Input ───────────────────────────────────────────────────────────────────

type KeyEvent struct {
	Type KeyType
	Char rune
}

type KeyType int

const (
	KeyChar KeyType = iota
	KeyEnter
	KeyEscape
	KeyBackspace
	KeyTab
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyHome
	KeyEnd
	KeyPgUp
	KeyPgDn
	KeyDelete
	KeyCtrlC
	KeyCtrlQ
	KeyCtrlS
	KeyCtrlR
)

func ReadKey() KeyEvent {
	reader := bufio.NewReader(os.Stdin)
	b, err := reader.ReadByte()
	if err != nil {
		return KeyEvent{Type: KeyChar, Char: 0}
	}

	switch b {
	case 0:
		return KeyEvent{Type: KeyChar, Char: 0}
	case 3: // Ctrl-C
		return KeyEvent{Type: KeyCtrlC}
	case 17: // Ctrl-Q
		return KeyEvent{Type: KeyCtrlQ}
	case 18: // Ctrl-R
		return KeyEvent{Type: KeyCtrlR}
	case 19: // Ctrl-S
		return KeyEvent{Type: KeyCtrlS}
	case 9: // Tab
		return KeyEvent{Type: KeyTab}
	case 10, 13: // Enter
		return KeyEvent{Type: KeyEnter}
	case 27: // Escape or escape sequence
		b2, err := reader.ReadByte()
		if err != nil {
			return KeyEvent{Type: KeyEscape}
		}
		if b2 == '[' {
			b3, err := reader.ReadByte()
			if err != nil {
				return KeyEvent{Type: KeyEscape}
			}
			switch b3 {
			case 'A':
				return KeyEvent{Type: KeyUp}
			case 'B':
				return KeyEvent{Type: KeyDown}
			case 'C':
				return KeyEvent{Type: KeyRight}
			case 'D':
				return KeyEvent{Type: KeyLeft}
			case 'H':
				return KeyEvent{Type: KeyHome}
			case 'F':
				return KeyEvent{Type: KeyEnd}
			case '3':
				reader.ReadByte() // consume ~
				return KeyEvent{Type: KeyDelete}
			case '5':
				reader.ReadByte()
				return KeyEvent{Type: KeyPgUp}
			case '6':
				reader.ReadByte()
				return KeyEvent{Type: KeyPgDn}
			}
			return KeyEvent{Type: KeyEscape}
		}
		return KeyEvent{Type: KeyEscape}
	case 127: // Backspace
		return KeyEvent{Type: KeyBackspace}
	default:
		return KeyEvent{Type: KeyChar, Char: rune(b)}
	}
}

// ─── Drawing Helpers ─────────────────────────────────────────────────────────

// Pad or truncate string to exact width
func pad(s string, w int) string {
	runes := []rune(s)
	if len(runes) >= w {
		if w > 3 {
			return string(runes[:w-1]) + "…"
		}
		return string(runes[:w])
	}
	return s + strings.Repeat(" ", w-len(runes))
}

// Center a string within width
func center(s string, w int) string {
	runes := []rune(s)
	if len(runes) >= w {
		return string(runes[:w])
	}
	left := (w - len(runes)) / 2
	right := w - len(runes) - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

// Repeat a character
func rep(ch string, n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(ch, n)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
