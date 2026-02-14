package main

import (
	"fmt"
	"strings"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// App — Main TUI application state and orchestrator
// ═══════════════════════════════════════════════════════════════════════════════

type Tab int

const (
	TabProfile Tab = iota
	TabKeyboard
	TabAura
	TabBattery
	TabFans
	TabBios
	TabConsole
	TabCount
)

var tabNames = []string{
	"Profile", "Keyboard", "Aura RGB", "Battery", "Fans", "BIOS", "Console",
}

var tabKeys = []string{
	"1", "2", "3", "4", "5", "6", "7",
}

type App struct {
	term    *Terminal
	backend *Backend
	running bool

	// Navigation
	activeTab Tab
	focusIdx  int // per-tab focus index

	// State
	profile       string
	kbdLevel      int // 0=off,1=low,2=med,3=high
	auraMode      int
	auraSection   int // 0=modes, 1=colour1, 2=colour2, 3=speed
	auraColour1   int // index into auraColours
	auraColour2   int
	auraSpeed     int // 0=low, 1=med, 2=high
	chargeLimit   int
	oneShotCharge bool

	// Fan curve
	selectedFan   int // 0=CPU, 1=GPU
	fanSpeeds     [2][8]int
	fanTemps      [8]int
	fanEnabled    bool
	fanFocusPoint int

	// BIOS
	panelOverdrive  bool
	gpuMuxDedicated bool

	// Console
	consoleInput  string
	consoleLog    []ConsoleLine
	consoleScroll int

	// Status
	installed  bool
	statusMsg  string
	statusTime time.Time
	statusOk   bool
}

type ConsoleLine struct {
	Time    string
	Command string
	Output  string
	Ok      bool
}

var kbdLabels = []string{"Off", "Low", "Med", "High"}
var kbdValues = []string{"off", "low", "med", "high"}

var auraModes = []string{
	"Static", "Breathe", "Rainbow Cycle", "Rainbow Wave", "Stars", "Rain",
	"Highlight", "Laser", "Ripple", "Pulse", "Comet", "Flash",
}

type AuraColour struct {
	Name string
	Hex  string
	Rgb  Color
}

var auraColours = []AuraColour{
	{"Red", "ff0000", Color{255, 0, 0}},
	{"Orange", "ff6600", Color{255, 102, 0}},
	{"Yellow", "ffcc00", Color{255, 204, 0}},
	{"Green", "00ff00", Color{0, 255, 0}},
	{"Cyan", "00ffff", Color{0, 255, 255}},
	{"Blue", "0000ff", Color{0, 0, 255}},
	{"Purple", "8800ff", Color{136, 0, 255}},
	{"Pink", "ff00aa", Color{255, 0, 170}},
	{"White", "ffffff", Color{255, 255, 255}},
}

var auraSpeeds = []string{"low", "med", "high"}
var auraSpeedLabels = []string{"Low", "Med", "High"}

// auraEffectNeedsColour1 returns true if the effect uses --colour
func auraEffectNeedsColour1(mode string) bool {
	switch mode {
	case "Rainbow Cycle", "Rainbow Wave", "Rain":
		return false
	}
	return true
}

// auraEffectNeedsColour2 returns true if the effect uses --colour2
func auraEffectNeedsColour2(mode string) bool {
	return mode == "Breathe" || mode == "Stars"
}

// auraEffectNeedsSpeed returns true if the effect uses --speed
func auraEffectNeedsSpeed(mode string) bool {
	switch mode {
	case "Static", "Pulse", "Comet", "Flash":
		return false
	}
	return true
}

func NewApp(term *Terminal, backend *Backend) *App {
	a := &App{
		term:        term,
		backend:     backend,
		running:     true,
		activeTab:   TabProfile,
		profile:     "Balanced",
		kbdLevel:    2,
		chargeLimit: 80,
		auraSpeed:   1, // med
		auraColour2: 4, // cyan (contrast with default red)
		fanTemps:    [8]int{30, 40, 50, 60, 70, 80, 90, 100},
	}
	// Default fan curves
	a.fanSpeeds[0] = [8]int{0, 5, 10, 20, 35, 55, 65, 65} // CPU
	a.fanSpeeds[1] = [8]int{0, 5, 10, 15, 30, 50, 60, 60} // GPU
	return a
}

func (a *App) Init() {
	a.installed = a.backend.IsInstalled()
	if a.installed {
		a.profile = a.backend.GetProfile()
		kbd := a.backend.GetKbdBrightness()
		for i, v := range kbdValues {
			if v == kbd {
				a.kbdLevel = i
				break
			}
		}
		a.chargeLimit = a.backend.GetChargeLimit()
		if aura := a.backend.GetAuraState(); aura != nil {
			a.initAuraState(aura)
		}
	}
}

func (a *App) initAuraState(aura *AuraState) {
	// Map config mode names (e.g. "RainbowCycle") to display names ("Rainbow Cycle")
	modeMap := map[string]string{
		"RainbowCycle": "Rainbow Cycle",
		"RainbowWave":  "Rainbow Wave",
	}
	displayMode := aura.Mode
	if mapped, ok := modeMap[aura.Mode]; ok {
		displayMode = mapped
	}
	for i, m := range auraModes {
		if m == displayMode {
			a.auraMode = i
			break
		}
	}

	a.auraColour1 = closestAuraColour(aura.R1, aura.G1, aura.B1)
	a.auraColour2 = closestAuraColour(aura.R2, aura.G2, aura.B2)

	speedLo := strings.ToLower(aura.Speed)
	for i, s := range auraSpeeds {
		if s == speedLo {
			a.auraSpeed = i
			break
		}
	}
}

func closestAuraColour(r, g, b int) int {
	best := 0
	bestDist := 1<<31 - 1
	for i, c := range auraColours {
		dr := int(c.Rgb.R) - r
		dg := int(c.Rgb.G) - g
		db := int(c.Rgb.B) - b
		dist := dr*dr + dg*dg + db*db
		if dist < bestDist {
			bestDist = dist
			best = i
		}
	}
	return best
}

func (a *App) SetStatus(msg string, ok bool) {
	a.statusMsg = msg
	a.statusOk = ok
	a.statusTime = time.Now()
}

func (a *App) addLog(cmd, output string, ok bool) {
	a.consoleLog = append(a.consoleLog, ConsoleLine{
		Time:    time.Now().Format("15:04:05"),
		Command: cmd,
		Output:  output,
		Ok:      ok,
	})
	// Keep last 100 lines
	if len(a.consoleLog) > 100 {
		a.consoleLog = a.consoleLog[len(a.consoleLog)-100:]
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Render — full screen redraw
// ═══════════════════════════════════════════════════════════════════════════════

func (a *App) Render() {
	t := a.term
	t.updateSize()
	t.Clear()

	W := t.Width()

	// Background
	t.FillRect(0, 0, W, t.Height(), ColBg)

	// ─── Header ──────────────────────────────────────────────────────────
	t.ResetStyle()
	t.Bg(ColPanel)
	t.MoveTo(0, 0)
	t.Write(rep(" ", W))

	t.ResetStyle()
	t.Bold()
	t.Bg(ColAccent)
	t.Fg(Color{255, 255, 255})
	t.MoveTo(1, 0)
	t.Write(" R ")

	t.ResetStyle()
	t.Bg(ColPanel)
	t.Bold()
	t.Fg(ColText)
	t.MoveTo(5, 0)
	t.Write("AsusCtl Control Center")

	// Status indicator (right side)
	statusStr := "● connected"
	statusCol := ColSuccess
	if !a.installed {
		statusStr = "● asusctl not found"
		statusCol = ColError
	}
	t.Fg(statusCol)
	t.MoveTo(W-len(statusStr)-2, 0)
	t.Write(statusStr)

	// ─── Tab bar ─────────────────────────────────────────────────────────
	t.ResetStyle()
	t.Bg(ColPanel)
	t.MoveTo(0, 1)
	t.Write(rep(" ", W))

	x := 1
	for i := 0; i < int(TabCount); i++ {
		label := fmt.Sprintf(" %s:%s ", tabKeys[i], tabNames[i])
		if Tab(i) == a.activeTab {
			t.ResetStyle()
			t.Bold()
			t.Bg(ColAccent)
			t.Fg(Color{255, 255, 255})
		} else {
			t.ResetStyle()
			t.Bg(ColPanel)
			t.Fg(ColTextDim)
		}
		t.MoveTo(x, 1)
		t.Write(label)
		x += len(label) + 1
	}

	// ─── Separator ───────────────────────────────────────────────────────
	t.ResetStyle()
	t.Fg(ColBorder)
	t.MoveTo(0, 2)
	t.Write(rep("─", W))

	// ─── Content area ────────────────────────────────────────────────────
	contentY := 3
	contentH := t.Height() - 5 // Leave room for footer

	switch a.activeTab {
	case TabProfile:
		a.renderProfile(contentY, contentH)
	case TabKeyboard:
		a.renderKeyboard(contentY, contentH)
	case TabAura:
		a.renderAura(contentY, contentH)
	case TabBattery:
		a.renderBattery(contentY, contentH)
	case TabFans:
		a.renderFans(contentY, contentH)
	case TabBios:
		a.renderBios(contentY, contentH)
	case TabConsole:
		a.renderConsole(contentY, contentH)
	}

	// ─── Footer / status bar ─────────────────────────────────────────────
	footerY := t.Height() - 2

	t.ResetStyle()
	t.Fg(ColBorder)
	t.MoveTo(0, footerY)
	t.Write(rep("─", W))

	t.ResetStyle()
	t.Bg(ColPanel)
	t.MoveTo(0, footerY+1)
	t.Write(rep(" ", W))

	// Help text
	t.Fg(ColTextDim)
	t.MoveTo(1, footerY+1)
	t.Write("1-7:Tab  ↑↓:Navigate  ←→:Adjust  Enter:Apply  q:Quit")

	// Status message (right side)
	if a.statusMsg != "" && time.Since(a.statusTime) < 4*time.Second {
		sc := ColSuccess
		if !a.statusOk {
			sc = ColError
		}
		msg := a.statusMsg
		if len(msg) > 40 {
			msg = msg[:39] + "…"
		}
		t.Fg(sc)
		t.MoveTo(W-len(msg)-2, footerY+1)
		t.Write(msg)
	}

	t.ResetStyle()
	t.Flush()
}

// ═══════════════════════════════════════════════════════════════════════════════
// Page: Profile
// ═══════════════════════════════════════════════════════════════════════════════

func (a *App) renderProfile(y, h int) {
	t := a.term
	W := t.Width()
	cx := 3 // content x offset

	t.TextBold(cx, y+1, ColText, "Power Profile")
	t.Text(cx, y+2, ColTextDim, "Select a performance mode for your laptop")

	profiles := []struct {
		name  string
		icon  string
		desc  string
		color Color
	}{
		{"Performance", "⚡", "Maximum clocks, aggressive fans", ColPerf},
		{"Balanced", "⚖", "Auto-tuned balance of speed & efficiency", ColBal},
		{"Quiet", "🔇", "Minimal fan noise, power saving", ColQuiet},
	}

	for i, p := range profiles {
		row := y + 4 + i*3
		selected := a.profile == p.name
		focused := a.focusIdx == i

		if selected {
			t.ResetStyle()
			t.Bg(Color{p.color.R / 6, p.color.G / 6, p.color.B / 6})
			t.MoveTo(cx, row)
			t.Write(rep(" ", min(W-6, 60)))
			t.MoveTo(cx, row+1)
			t.Write(rep(" ", min(W-6, 60)))

			t.Fg(p.color)
			t.Bold()
			t.MoveTo(cx+1, row)
			if focused {
				t.Write("▸ ")
			} else {
				t.Write("● ")
			}
			t.Write(p.icon + " " + p.name)
			t.ResetStyle()
			t.Fg(ColTextDim)
			t.Bg(Color{p.color.R / 6, p.color.G / 6, p.color.B / 6})
			t.MoveTo(cx+3, row+1)
			t.Write(p.desc)

			// Active marker
			activeStr := " ACTIVE "
			t.ResetStyle()
			t.Bg(p.color)
			t.Fg(Color{255, 255, 255})
			t.Bold()
			t.MoveTo(min(W-6, 60)+cx-len(activeStr)-1, row)
			t.Write(activeStr)
		} else {
			t.ResetStyle()
			if focused {
				t.Fg(ColText)
				t.MoveTo(cx+1, row)
				t.Write("▸ " + p.icon + " " + p.name)
			} else {
				t.Fg(ColTextDim)
				t.MoveTo(cx+1, row)
				t.Write("  " + p.icon + " " + p.name)
			}
			t.Fg(ColTextMut)
			t.MoveTo(cx+3, row+1)
			t.Write(p.desc)
		}
	}

	t.ResetStyle()
	t.Fg(ColTextMut)
	t.MoveTo(cx, y+4+9+1)
	t.Write("Press Enter to switch profile, or ↑/↓ to navigate")
}

func (a *App) handleProfile(key KeyEvent) {
	switch key.Type {
	case KeyUp:
		a.focusIdx = (a.focusIdx + 2) % 3
	case KeyDown:
		a.focusIdx = (a.focusIdx + 1) % 3
	case KeyEnter:
		profiles := []string{"Performance", "Balanced", "Quiet"}
		p := profiles[a.focusIdx]
		ok, out := a.backend.SetProfile(p)
		if ok {
			a.profile = p
			a.SetStatus("Profile → "+p, true)
		} else {
			a.SetStatus("Failed: "+out, false)
		}
		a.addLog("profile --profile-set "+p, out, ok)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Page: Keyboard
// ═══════════════════════════════════════════════════════════════════════════════

func (a *App) renderKeyboard(y, h int) {
	t := a.term
	cx := 3

	t.TextBold(cx, y+1, ColText, "Keyboard Backlight")
	t.Text(cx, y+2, ColTextDim, "Adjust keyboard backlight brightness level")

	for i, label := range kbdLabels {
		row := y + 4 + i*2
		selected := a.kbdLevel == i
		focused := a.focusIdx == i

		// Draw bar segments to visualize brightness
		barLen := i * 6

		if selected {
			t.ResetStyle()
			t.Bold()
			t.Fg(ColAccent)
			t.MoveTo(cx+1, row)
			if focused {
				t.Write("▸ ● " + label)
			} else {
				t.Write("  ● " + label)
			}
			t.Fg(ColAccent)
			t.MoveTo(cx+14, row)
			t.Write(rep("█", barLen))
			t.Fg(ColTextMut)
			t.Write(rep("░", 18-barLen))

			t.Fg(ColTextDim)
			t.MoveTo(cx+35, row)
			t.Write("ACTIVE")
		} else {
			t.ResetStyle()
			if focused {
				t.Fg(ColText)
				t.MoveTo(cx+1, row)
				t.Write("▸ ○ " + label)
			} else {
				t.Fg(ColTextDim)
				t.MoveTo(cx+1, row)
				t.Write("  ○ " + label)
			}
			t.Fg(ColTextMut)
			t.MoveTo(cx+14, row)
			t.Write(rep("░", barLen))
		}
	}

	t.Text(cx, y+13, ColTextMut, "Enter to set brightness")
}

func (a *App) handleKeyboard(key KeyEvent) {
	switch key.Type {
	case KeyUp:
		a.focusIdx = (a.focusIdx + 3) % 4
	case KeyDown:
		a.focusIdx = (a.focusIdx + 1) % 4
	case KeyEnter:
		ok, out := a.backend.SetKbdBrightness(kbdValues[a.focusIdx])
		if ok {
			a.kbdLevel = a.focusIdx
			a.SetStatus("Keyboard → "+kbdLabels[a.focusIdx], true)
		} else {
			a.SetStatus("Failed: "+out, false)
		}
		a.addLog("--kbd-bright "+kbdValues[a.focusIdx], out, ok)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Page: Aura RGB
// ═══════════════════════════════════════════════════════════════════════════════

func (a *App) renderAura(y, h int) {
	t := a.term
	W := t.Width()
	cx := 3

	t.TextBold(cx, y+1, ColAura, "Aura RGB Lighting")
	t.Text(cx, y+2, ColTextDim, "Choose effect, colour, and speed")

	cols := 3
	if W > 80 {
		cols = 4
	}

	// ─── Mode grid ───
	for i, mode := range auraModes {
		col := i % cols
		row := i / cols
		px := cx + col*18
		py := y + 4 + row*2

		selected := a.auraMode == i
		focused := a.auraSection == 0 && a.focusIdx == i

		w := 16
		label := center(mode, w)

		if selected {
			t.ResetStyle()
			t.Bg(Color{ColAura.R / 4, ColAura.G / 4, ColAura.B / 4})
			t.Fg(Color{200, 160, 255})
			t.Bold()
			t.MoveTo(px, py)
			if focused {
				t.Write("▸" + label)
			} else {
				t.Write(" " + label)
			}
		} else if focused {
			t.ResetStyle()
			t.Fg(ColText)
			t.MoveTo(px, py)
			t.Write("▸" + pad(mode, w))
		} else {
			t.ResetStyle()
			t.Fg(ColTextDim)
			t.MoveTo(px, py)
			t.Write(" " + pad(mode, w))
		}
	}

	modeRows := (len(auraModes)-1)/cols + 1
	sectionY := y + 4 + modeRows*2 + 1
	curMode := auraModes[a.auraMode]

	// ─── Colour 1 ───
	if auraEffectNeedsColour1(curMode) {
		t.Text(cx, sectionY, ColTextDim, "Colour:")
		for i, c := range auraColours {
			px := cx + 9 + i*4
			focused := a.auraSection == 1 && a.focusIdx == i
			selected := a.auraColour1 == i
			t.ResetStyle()
			t.Bg(c.Rgb)
			if focused {
				t.Fg(Color{0, 0, 0})
				t.Bold()
				t.MoveTo(px, sectionY)
				if selected {
					t.Write("▸◆ ")
				} else {
					t.Write("▸  ")
				}
			} else {
				t.MoveTo(px, sectionY)
				if selected {
					t.Fg(Color{0, 0, 0})
					t.Bold()
					t.Write(" ◆ ")
				} else {
					t.Write("   ")
				}
			}
		}
		t.ResetStyle()
		sectionY += 2
	}

	// ─── Colour 2 ───
	if auraEffectNeedsColour2(curMode) {
		t.Text(cx, sectionY, ColTextDim, "Colour2:")
		for i, c := range auraColours {
			px := cx + 9 + i*4
			focused := a.auraSection == 2 && a.focusIdx == i
			selected := a.auraColour2 == i
			t.ResetStyle()
			t.Bg(c.Rgb)
			if focused {
				t.Fg(Color{0, 0, 0})
				t.Bold()
				t.MoveTo(px, sectionY)
				if selected {
					t.Write("▸◆ ")
				} else {
					t.Write("▸  ")
				}
			} else {
				t.MoveTo(px, sectionY)
				if selected {
					t.Fg(Color{0, 0, 0})
					t.Bold()
					t.Write(" ◆ ")
				} else {
					t.Write("   ")
				}
			}
		}
		t.ResetStyle()
		sectionY += 2
	}

	// ─── Speed ───
	if auraEffectNeedsSpeed(curMode) {
		t.Text(cx, sectionY, ColTextDim, "Speed:  ")
		for i, label := range auraSpeedLabels {
			px := cx + 9 + i*8
			focused := a.auraSection == 3 && a.focusIdx == i
			selected := a.auraSpeed == i
			if selected {
				t.ResetStyle()
				t.Bg(ColAura)
				t.Fg(Color{255, 255, 255})
				t.Bold()
				t.MoveTo(px, sectionY)
				if focused {
					t.Write("▸" + label + " ")
				} else {
					t.Write(" " + label + " ")
				}
			} else if focused {
				t.ResetStyle()
				t.Fg(ColText)
				t.MoveTo(px, sectionY)
				t.Write("▸" + label + " ")
			} else {
				t.ResetStyle()
				t.Fg(ColTextDim)
				t.MoveTo(px, sectionY)
				t.Write(" " + label + " ")
			}
		}
		t.ResetStyle()
		sectionY += 2
	}

	t.Text(cx, sectionY, ColTextMut, "Enter to apply  │  ↑/↓ sections  │  ←/→ select")
}

// auraSections returns which sections are active for the current mode
func (a *App) auraSections() []int {
	mode := auraModes[a.auraMode]
	sections := []int{0} // mode grid always present
	if auraEffectNeedsColour1(mode) {
		sections = append(sections, 1)
	}
	if auraEffectNeedsColour2(mode) {
		sections = append(sections, 2)
	}
	if auraEffectNeedsSpeed(mode) {
		sections = append(sections, 3)
	}
	return sections
}

func (a *App) auraClampSection() {
	sections := a.auraSections()
	found := false
	for _, s := range sections {
		if s == a.auraSection {
			found = true
			break
		}
	}
	if !found {
		a.auraSection = 0
		a.focusIdx = a.auraMode
	}
}

func (a *App) handleAura(key KeyEvent) {
	cols := 3
	if a.term.Width() > 80 {
		cols = 4
	}

	switch key.Type {
	case KeyUp:
		sections := a.auraSections()
		cur := -1
		for i, s := range sections {
			if s == a.auraSection {
				cur = i
				break
			}
		}
		if cur > 0 {
			a.auraSection = sections[cur-1]
			switch a.auraSection {
			case 0:
				a.focusIdx = a.auraMode
			case 1:
				a.focusIdx = a.auraColour1
			case 2:
				a.focusIdx = a.auraColour2
			case 3:
				a.focusIdx = a.auraSpeed
			}
		} else if a.auraSection == 0 {
			// Navigate within mode grid
			a.focusIdx -= cols
			if a.focusIdx < 0 {
				a.focusIdx += len(auraModes)
				if a.focusIdx >= len(auraModes) {
					a.focusIdx = len(auraModes) - 1
				}
			}
		}
	case KeyDown:
		sections := a.auraSections()
		cur := -1
		for i, s := range sections {
			if s == a.auraSection {
				cur = i
				break
			}
		}
		if a.auraSection == 0 {
			// Try moving down in the grid first
			next := a.focusIdx + cols
			if next < len(auraModes) {
				a.focusIdx = next
			} else if cur < len(sections)-1 {
				// Move to next section
				a.auraSection = sections[cur+1]
				switch a.auraSection {
				case 1:
					a.focusIdx = a.auraColour1
				case 2:
					a.focusIdx = a.auraColour2
				case 3:
					a.focusIdx = a.auraSpeed
				}
			}
		} else if cur < len(sections)-1 {
			a.auraSection = sections[cur+1]
			switch a.auraSection {
			case 1:
				a.focusIdx = a.auraColour1
			case 2:
				a.focusIdx = a.auraColour2
			case 3:
				a.focusIdx = a.auraSpeed
			}
		}
	case KeyLeft:
		switch a.auraSection {
		case 0:
			a.focusIdx = (a.focusIdx + len(auraModes) - 1) % len(auraModes)
		case 1:
			a.focusIdx = (a.focusIdx + len(auraColours) - 1) % len(auraColours)
		case 2:
			a.focusIdx = (a.focusIdx + len(auraColours) - 1) % len(auraColours)
		case 3:
			a.focusIdx = (a.focusIdx + len(auraSpeeds) - 1) % len(auraSpeeds)
		}
	case KeyRight:
		switch a.auraSection {
		case 0:
			a.focusIdx = (a.focusIdx + 1) % len(auraModes)
		case 1:
			a.focusIdx = (a.focusIdx + 1) % len(auraColours)
		case 2:
			a.focusIdx = (a.focusIdx + 1) % len(auraColours)
		case 3:
			a.focusIdx = (a.focusIdx + 1) % len(auraSpeeds)
		}
	case KeyEnter:
		switch a.auraSection {
		case 0:
			a.auraMode = a.focusIdx
			a.auraClampSection()
		case 1:
			a.auraColour1 = a.focusIdx
		case 2:
			a.auraColour2 = a.focusIdx
		case 3:
			a.auraSpeed = a.focusIdx
		}
		// Apply the effect
		mode := auraModes[a.auraMode]
		colour1 := ""
		colour2 := ""
		speed := ""
		if auraEffectNeedsColour1(mode) {
			colour1 = auraColours[a.auraColour1].Hex
		}
		if auraEffectNeedsColour2(mode) {
			colour2 = auraColours[a.auraColour2].Hex
		}
		if auraEffectNeedsSpeed(mode) {
			speed = auraSpeeds[a.auraSpeed]
		}
		ok, out := a.backend.SetAuraMode(mode, colour1, colour2, speed)
		if ok {
			a.SetStatus("Aura → "+mode, true)
		} else {
			a.SetStatus("Failed: "+out, false)
		}
		subcmd := strings.ToLower(strings.ReplaceAll(mode, " ", "-"))
		a.addLog("aura effect "+subcmd, out, ok)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Page: Battery
// ═══════════════════════════════════════════════════════════════════════════════

func (a *App) renderBattery(y, h int) {
	t := a.term
	W := t.Width()
	cx := 3

	t.TextBold(cx, y+1, ColText, "Battery & Charging")

	// Charge limit slider
	t.Text(cx, y+3, ColTextDim, "Charge Limit")

	barW := min(W-20, 50)
	pct := float64(a.chargeLimit-20) / 80.0

	t.MoveTo(cx, y+5)
	t.ResetStyle()

	// Draw slider track
	filled := int(pct * float64(barW))
	t.Bg(ColAccent)
	t.Write(rep(" ", filled))
	t.Bg(ColInput)
	t.Write(rep(" ", barW-filled))
	t.ResetStyle()

	// Value
	t.Bold()
	valStr := fmt.Sprintf(" %d%%", a.chargeLimit)
	if a.chargeLimit <= 60 {
		t.Fg(ColSuccess)
	} else if a.chargeLimit <= 80 {
		t.Fg(ColBal)
	} else {
		t.Fg(ColWarning)
	}
	t.Write(valStr)

	// Focus indicator
	if a.focusIdx == 0 {
		t.Fg(ColAccent)
		t.MoveTo(cx-2, y+5)
		t.Write("▸")
	}

	// Help text
	t.Text(cx, y+7, ColTextMut, "←/→ adjust by 5%  │  Enter to apply")

	// Recommendations
	t.Text(cx, y+9, ColTextDim, "Recommendations:")
	t.Text(cx+2, y+10, ColTextMut, "60% — Laptop always plugged in")
	t.Text(cx+2, y+11, ColTextMut, "75% — Unplugged regularly")
	t.Text(cx+2, y+12, ColTextMut, "80% — Good general default")

	// One-shot charge
	t.ResetStyle()
	t.HLine(cx, y+14, min(W-6, 50), ColBorder)

	focused1 := a.focusIdx == 1
	t.Text(cx, y+16, ColTextDim, "One-Shot Full Charge")
	t.Text(cx, y+17, ColTextMut, "Temporarily charge to 100% (once)")

	if focused1 {
		t.TextBold(cx-2, y+16, ColAccent, "▸")
	}

	t.MoveTo(cx+30, y+16)
	a.term.DrawButton(cx+30, y+16, "Toggle", focused1, ColAccent)
}

func (a *App) handleBattery(key KeyEvent) {
	switch key.Type {
	case KeyUp:
		a.focusIdx = 0
	case KeyDown:
		a.focusIdx = 1
	case KeyLeft:
		if a.focusIdx == 0 {
			a.chargeLimit = clamp(a.chargeLimit-5, 20, 100)
		}
	case KeyRight:
		if a.focusIdx == 0 {
			a.chargeLimit = clamp(a.chargeLimit+5, 20, 100)
		}
	case KeyEnter:
		if a.focusIdx == 0 {
			ok, out := a.backend.SetChargeLimit(a.chargeLimit)
			if ok {
				a.SetStatus(fmt.Sprintf("Charge limit → %d%%", a.chargeLimit), true)
			} else {
				a.SetStatus("Failed: "+out, false)
			}
			a.addLog(fmt.Sprintf("--chg-limit %d", a.chargeLimit), out, ok)
		} else {
			ok, out := a.backend.ToggleOneShotCharge()
			if ok {
				a.SetStatus("One-shot charge toggled", true)
			} else {
				a.SetStatus("Failed: "+out, false)
			}
			a.addLog("--one-shot-chg", out, ok)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Page: Fans
// ═══════════════════════════════════════════════════════════════════════════════

var fanPresets = map[string][8]int{
	"silent":      {0, 0, 0, 10, 20, 35, 45, 50},
	"balanced":    {0, 5, 10, 20, 35, 55, 65, 65},
	"performance": {15, 25, 35, 50, 65, 80, 90, 100},
	"full":        {100, 100, 100, 100, 100, 100, 100, 100},
}

func (a *App) renderFans(y, h int) {
	t := a.term
	W := t.Width()
	cx := 3

	t.TextBold(cx, y+1, ColText, "Fan Curve Editor")

	// Fan selector
	cpuActive := a.selectedFan == 0
	gpuActive := a.selectedFan == 1

	t.MoveTo(cx, y+3)
	t.ResetStyle()
	t.Write("Fan: ")
	a.term.DrawButton(cx+5, y+3, "CPU", cpuActive, ColAccent)
	a.term.DrawButton(cx+13, y+3, "GPU", gpuActive, ColAccent)

	// Custom curves toggle
	a.term.DrawToggle(cx+24, y+3, a.fanEnabled)
	t.Text(cx+33, y+3, ColTextDim, "Custom curves")

	// Fan curve ASCII graph
	graphX := cx + 5
	graphY := y + 5
	graphW := min(W-14, 56)
	graphH := min(h-12, 12)
	speeds := a.fanSpeeds[a.selectedFan]

	// Y axis labels
	for row := 0; row <= graphH; row++ {
		pct := 100 - (row * 100 / graphH)
		t.Fg(ColTextMut)
		t.MoveTo(cx, graphY+row)
		t.Write(fmt.Sprintf("%3d%%", pct))
	}

	// Draw grid + curve
	for row := 0; row <= graphH; row++ {
		pct := 100 - (row * 100 / graphH)
		t.MoveTo(graphX, graphY+row)
		for col := 0; col < graphW; col++ {
			pointIdx := col * 7 / (graphW - 1)
			if pointIdx >= 7 {
				pointIdx = 7
			}
			// Interpolate fan speed at this column
			frac := float64(col) / float64(graphW-1) * 7.0
			idx := int(frac)
			if idx >= 7 {
				idx = 6
			}
			rem := frac - float64(idx)
			spd := float64(speeds[idx])*(1-rem) + float64(speeds[idx+1])*rem
			spdRow := int((100 - spd) * float64(graphH) / 100.0)

			isPoint := false
			for p := 0; p < 8; p++ {
				px := p * (graphW - 1) / 7
				py := int((100 - float64(speeds[p])) * float64(graphH) / 100.0)
				if col == px && row == py {
					isPoint = true
					if a.focusIdx == p {
						t.ResetStyle()
						t.Bold()
						t.Fg(Color{255, 255, 255})
						t.Bg(ColAccent)
						t.Write("◆")
					} else {
						t.ResetStyle()
						t.Fg(ColAccent)
						t.Write("●")
					}
					break
				}
			}
			if isPoint {
				continue
			}

			if row == spdRow {
				t.ResetStyle()
				t.Fg(ColAccent)
				t.Write("─")
			} else if row > spdRow && pct%25 == 0 {
				t.ResetStyle()
				t.Fg(ColTextMut)
				t.Write("┄")
			} else if row > spdRow {
				t.ResetStyle()
				t.Fg(Color{ColAccent.R / 8, ColAccent.G / 8, ColAccent.B / 8})
				t.Write("░")
			} else {
				t.ResetStyle()
				t.Write(" ")
			}
		}
	}

	// X axis labels
	t.Fg(ColTextMut)
	for p := 0; p < 8; p++ {
		px := graphX + p*(graphW-1)/7
		t.MoveTo(px-1, graphY+graphH+1)
		t.Write(fmt.Sprintf("%d°", a.fanTemps[p]))
	}

	// Point value display
	infoY := graphY + graphH + 3
	t.Text(cx, infoY, ColTextDim,
		fmt.Sprintf("Point %d: %d°C → %d%%   (↑↓ speed, ←→ point, Tab fan, Enter apply)",
			a.focusIdx+1, a.fanTemps[a.focusIdx], speeds[a.focusIdx]))

	// Presets
	t.Text(cx, infoY+2, ColTextDim, "Presets:  s=Silent  b=Balanced  p=Performance  f=Full")

	// Current data string
	t.Fg(ColTextMut)
	t.MoveTo(cx, infoY+3)
	t.Write("Data: " + FormatFanCurve(a.fanTemps[:], speeds[:]))
}

func (a *App) handleFans(key KeyEvent) {
	speeds := &a.fanSpeeds[a.selectedFan]

	switch key.Type {
	case KeyUp:
		speeds[a.focusIdx] = clamp(speeds[a.focusIdx]+5, 0, 100)
	case KeyDown:
		speeds[a.focusIdx] = clamp(speeds[a.focusIdx]-5, 0, 100)
	case KeyLeft:
		a.focusIdx = (a.focusIdx + 7) % 8
	case KeyRight:
		a.focusIdx = (a.focusIdx + 1) % 8
	case KeyTab:
		a.selectedFan = (a.selectedFan + 1) % 2
	case KeyEnter:
		data := FormatFanCurve(a.fanTemps[:], speeds[:])
		fan := "cpu"
		if a.selectedFan == 1 {
			fan = "gpu"
		}
		ok, out := a.backend.SetFanCurve(fan, a.profile, data)
		if ok {
			a.SetStatus(fmt.Sprintf("Fan curve applied (%s)", strings.ToUpper(fan)), true)
		} else {
			a.SetStatus("Failed: "+out, false)
		}
		a.addLog("fan-curve --fan "+fan+" --data "+data, out, ok)
	case KeyChar:
		switch key.Char {
		case 's':
			a.fanSpeeds[a.selectedFan] = fanPresets["silent"]
			a.SetStatus("Preset: Silent", true)
		case 'b':
			a.fanSpeeds[a.selectedFan] = fanPresets["balanced"]
			a.SetStatus("Preset: Balanced", true)
		case 'p':
			a.fanSpeeds[a.selectedFan] = fanPresets["performance"]
			a.SetStatus("Preset: Performance", true)
		case 'f':
			a.fanSpeeds[a.selectedFan] = fanPresets["full"]
			a.SetStatus("Preset: Full Speed", true)
		case 'e':
			a.fanEnabled = !a.fanEnabled
			ok, out := a.backend.EnableFanCurves(a.profile, a.fanEnabled)
			if ok {
				st := "disabled"
				if a.fanEnabled {
					st = "enabled"
				}
				a.SetStatus("Custom fan curves "+st, true)
			} else {
				a.SetStatus("Failed: "+out, false)
			}
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Page: BIOS
// ═══════════════════════════════════════════════════════════════════════════════

func (a *App) renderBios(y, h int) {
	t := a.term
	cx := 3

	t.TextBold(cx, y+1, ColWarning, "⚠ BIOS / EFI Settings")
	t.Text(cx, y+2, ColTextDim, "Stored in UEFI variables. Changes may require a reboot.")

	// Panel overdrive
	row := y + 4
	focused0 := a.focusIdx == 0
	if focused0 {
		t.TextBold(cx, row, ColText, "▸ Panel Overdrive")
	} else {
		t.Text(cx, row, ColTextDim, "  Panel Overdrive")
	}
	t.Text(cx+2, row+1, ColTextMut, "Reduce ghosting (may introduce artifacts)")
	a.term.DrawToggle(cx+46, row, a.panelOverdrive)

	// GPU MUX
	row = y + 7
	focused1 := a.focusIdx == 1
	if focused1 {
		t.TextBold(cx, row, ColText, "▸ GPU MUX — Dedicated / G-Sync")
	} else {
		t.Text(cx, row, ColTextDim, "  GPU MUX — Dedicated / G-Sync")
	}
	t.Text(cx+2, row+1, ColTextMut, "Route display through dGPU only (requires reboot)")
	a.term.DrawToggle(cx+46, row, a.gpuMuxDedicated)

	t.Text(cx, y+11, ColTextMut, "Enter to toggle selected setting")
}

func (a *App) handleBios(key KeyEvent) {
	switch key.Type {
	case KeyUp:
		a.focusIdx = 0
	case KeyDown:
		a.focusIdx = 1
	case KeyEnter:
		if a.focusIdx == 0 {
			a.panelOverdrive = !a.panelOverdrive
			ok, out := a.backend.SetPanelOverdrive(a.panelOverdrive)
			if ok {
				st := "OFF"
				if a.panelOverdrive {
					st = "ON"
				}
				a.SetStatus("Panel overdrive → "+st, true)
			} else {
				a.SetStatus("Failed: "+out, false)
				a.panelOverdrive = !a.panelOverdrive // revert
			}
			a.addLog(fmt.Sprintf("armoury set panel_od %v", a.panelOverdrive), out, ok)
		} else {
			a.gpuMuxDedicated = !a.gpuMuxDedicated
			ok, out := a.backend.SetGpuMux(a.gpuMuxDedicated)
			if ok {
				st := "Hybrid"
				if a.gpuMuxDedicated {
					st = "Dedicated"
				}
				a.SetStatus("GPU MUX → "+st+" (reboot required)", true)
			} else {
				a.SetStatus("Failed: "+out, false)
				a.gpuMuxDedicated = !a.gpuMuxDedicated
			}
			a.addLog(fmt.Sprintf("armoury set gpu_mux_mode %v", a.gpuMuxDedicated), out, ok)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Page: Console
// ═══════════════════════════════════════════════════════════════════════════════

func (a *App) renderConsole(y, h int) {
	t := a.term
	W := t.Width()
	cx := 3

	t.TextBold(cx, y+1, ColText, "Raw Console")
	t.Text(cx, y+2, ColTextDim, "Run any asusctl command directly")

	// Input line
	t.Fg(ColTextDim)
	t.MoveTo(cx, y+4)
	t.Write("asusctl ")
	t.ResetStyle()
	t.Fg(ColText)
	t.Bg(ColInput)

	inputW := min(W-14, 60)
	display := a.consoleInput
	if len(display) > inputW-1 {
		display = display[len(display)-inputW+1:]
	}
	t.Write(pad(display, inputW))
	t.ResetStyle()
	t.Fg(ColTextMut)
	t.Write(" Enter")

	// Log area
	logY := y + 6
	logH := h - 7
	if logH < 3 {
		logH = 3
	}

	t.HLine(cx, logY, min(W-6, 70), ColBorder)

	visibleLines := logH
	start := len(a.consoleLog) - visibleLines - a.consoleScroll
	if start < 0 {
		start = 0
	}
	end := start + visibleLines
	if end > len(a.consoleLog) {
		end = len(a.consoleLog)
	}

	for i, lineIdx := start, 0; i < end; i++ {
		entry := a.consoleLog[i]
		row := logY + 1 + lineIdx

		t.Fg(ColTextMut)
		t.MoveTo(cx, row)
		t.Write(entry.Time + " ")

		t.Fg(ColAccent)
		t.Write("$ " + entry.Command)
		lineIdx++

		if entry.Output != "" && lineIdx < visibleLines {
			row = logY + 1 + lineIdx
			if entry.Ok {
				t.Fg(ColSuccess)
			} else {
				t.Fg(ColError)
			}
			out := entry.Output
			maxW := W - cx - 4
			if len(out) > maxW {
				out = out[:maxW-1] + "…"
			}
			t.MoveTo(cx+2, row)
			t.Write(out)
			lineIdx++
		}

		if lineIdx >= visibleLines {
			break
		}
	}

	if len(a.consoleLog) == 0 {
		t.Fg(ColTextMut)
		t.MoveTo(cx+2, logY+2)
		t.Write("No commands run yet. All command outputs appear here.")
	}
}

func (a *App) handleConsole(key KeyEvent) {
	switch key.Type {
	case KeyChar:
		if key.Char >= 32 && key.Char < 127 {
			a.consoleInput += string(key.Char)
		}
	case KeyBackspace:
		if len(a.consoleInput) > 0 {
			a.consoleInput = a.consoleInput[:len(a.consoleInput)-1]
		}
	case KeyEnter:
		if a.consoleInput != "" {
			cmd := a.consoleInput
			a.consoleInput = ""
			ok, out := a.backend.RunRaw(cmd)
			a.addLog(cmd, out, ok)
			if ok {
				a.SetStatus("Command OK", true)
			} else {
				a.SetStatus("Command failed", false)
			}
			a.consoleScroll = 0
		}
	case KeyPgUp:
		a.consoleScroll = min(a.consoleScroll+3, max(0, len(a.consoleLog)-5))
	case KeyPgDn:
		a.consoleScroll = max(a.consoleScroll-3, 0)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Input Dispatch
// ═══════════════════════════════════════════════════════════════════════════════

func (a *App) HandleKey(key KeyEvent) {
	// Global keys
	switch key.Type {
	case KeyCtrlC, KeyCtrlQ:
		a.running = false
		return
	case KeyChar:
		if key.Char == 'q' && a.activeTab != TabConsole {
			a.running = false
			return
		}
		// Tab switching with number keys (only outside console)
		if a.activeTab != TabConsole || a.consoleInput == "" {
			if key.Char >= '1' && key.Char <= '7' {
				newTab := Tab(key.Char - '1')
				if newTab != a.activeTab {
					a.activeTab = newTab
					a.focusIdx = 0
					a.auraSection = 0
				}
				return
			}
		}
	}

	// Per-tab handlers
	switch a.activeTab {
	case TabProfile:
		a.handleProfile(key)
	case TabKeyboard:
		a.handleKeyboard(key)
	case TabAura:
		a.handleAura(key)
	case TabBattery:
		a.handleBattery(key)
	case TabFans:
		a.handleFans(key)
	case TabBios:
		a.handleBios(key)
	case TabConsole:
		a.handleConsole(key)
	}
}
