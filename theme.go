package main

// ═══════════════════════════════════════════════════════════════════════════════
// Theme — colors and box-drawing primitives
// ═══════════════════════════════════════════════════════════════════════════════

// RGB color triplet
type Color struct{ R, G, B int }

var (
	ColBg       = Color{10, 10, 12}
	ColPanel    = Color{20, 20, 24}
	ColCard     = Color{28, 28, 32}
	ColInput    = Color{38, 38, 44}
	ColBorder   = Color{50, 50, 58}
	ColAccent   = Color{229, 24, 45}
	ColAccentDm = Color{140, 16, 28}
	ColText     = Color{228, 228, 231}
	ColTextDim  = Color{113, 113, 122}
	ColTextMut  = Color{63, 63, 70}
	ColSuccess  = Color{34, 197, 94}
	ColWarning  = Color{245, 158, 11}
	ColError    = Color{239, 68, 68}
	ColPerf     = Color{239, 68, 68}
	ColBal      = Color{59, 130, 246}
	ColQuiet    = Color{34, 197, 94}
	ColAura     = Color{168, 85, 247}
)

func (t *Terminal) Fg(c Color) { t.SetFg(c.R, c.G, c.B) }
func (t *Terminal) Bg(c Color) { t.SetBg(c.R, c.G, c.B) }

// ─── Box Drawing ─────────────────────────────────────────────────────────────

// Draw a box with single-line Unicode characters
func (t *Terminal) DrawBox(x, y, w, h int, border Color) {
	t.Fg(border)
	// Top
	t.MoveTo(x, y)
	t.Write("┌" + rep("─", w-2) + "┐")
	// Sides
	for row := 1; row < h-1; row++ {
		t.MoveTo(x, y+row)
		t.Write("│")
		t.MoveTo(x+w-1, y+row)
		t.Write("│")
	}
	// Bottom
	t.MoveTo(x, y+h-1)
	t.Write("└" + rep("─", w-2) + "┘")
}

// Fill a rectangular region with a background color
func (t *Terminal) FillRect(x, y, w, h int, bg Color) {
	t.Bg(bg)
	blank := rep(" ", w)
	for row := 0; row < h; row++ {
		t.MoveTo(x, y+row)
		t.Write(blank)
	}
}

// Draw a horizontal line
func (t *Terminal) HLine(x, y, w int, c Color) {
	t.Fg(c)
	t.MoveTo(x, y)
	t.Write(rep("─", w))
}

// Draw text at position with fg color
func (t *Terminal) Text(x, y int, fg Color, s string) {
	t.ResetStyle()
	t.Fg(fg)
	t.MoveTo(x, y)
	t.Write(s)
}

// Draw text with bg
func (t *Terminal) TextBg(x, y int, fg, bg Color, s string) {
	t.ResetStyle()
	t.Fg(fg)
	t.Bg(bg)
	t.MoveTo(x, y)
	t.Write(s)
}

// Draw bold text
func (t *Terminal) TextBold(x, y int, fg Color, s string) {
	t.ResetStyle()
	t.Bold()
	t.Fg(fg)
	t.MoveTo(x, y)
	t.Write(s)
}

// ─── Bar / Gauge drawing ─────────────────────────────────────────────────────

// Draw a horizontal progress bar
func (t *Terminal) DrawBar(x, y, w int, pct float64, fg, bg Color) {
	filled := int(pct * float64(w))
	filled = clamp(filled, 0, w)

	t.MoveTo(x, y)
	t.Bg(fg)
	t.Write(rep(" ", filled))
	t.Bg(bg)
	t.Write(rep(" ", w-filled))
	t.ResetStyle()
}

// Draw a labeled button
func (t *Terminal) DrawButton(x, y int, label string, selected bool, accent Color) {
	w := len([]rune(label)) + 4
	if selected {
		t.ResetStyle()
		t.Bg(accent)
		t.Fg(Color{255, 255, 255})
		t.Bold()
		t.MoveTo(x, y)
		t.Write(" " + label + " ")
	} else {
		t.ResetStyle()
		t.Fg(ColBorder)
		t.MoveTo(x, y)
		t.Write("[")
		t.Fg(ColTextDim)
		t.Write(label)
		t.Fg(ColBorder)
		t.Write("]")
	}
	_ = w
}

// Draw a toggle switch
func (t *Terminal) DrawToggle(x, y int, on bool) {
	if on {
		t.ResetStyle()
		t.Bg(ColAccent)
		t.Fg(Color{255, 255, 255})
		t.MoveTo(x, y)
		t.Write(" ◉ ON  ")
	} else {
		t.ResetStyle()
		t.Bg(ColInput)
		t.Fg(ColTextDim)
		t.MoveTo(x, y)
		t.Write(" ○ OFF ")
	}
	t.ResetStyle()
}
