package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// AsusCtl Backend — wraps the asusctl CLI
// ═══════════════════════════════════════════════════════════════════════════════

type Backend struct{}

func NewBackend() *Backend {
	return &Backend{}
}

func (b *Backend) run(args ...string) (bool, string) {
	cmd := exec.Command("asusctl", args...)
	done := make(chan struct {
		out []byte
		err error
	}, 1)

	go func() {
		out, err := cmd.CombinedOutput()
		done <- struct {
			out []byte
			err error
		}{out, err}
	}()

	select {
	case r := <-done:
		output := strings.TrimSpace(string(r.out))
		return r.err == nil, output
	case <-time.After(5 * time.Second):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return false, "command timed out"
	}
}

func (b *Backend) IsInstalled() bool {
	_, err := exec.LookPath("asusctl")
	return err == nil
}

// ─── Profile ─────────────────────────────────────────────────────────────────

func (b *Backend) GetProfile() string {
	ok, out := b.run("profile", "get")
	if ok {
		lo := strings.ToLower(out)
		if strings.Contains(lo, "performance") {
			return "Performance"
		} else if strings.Contains(lo, "balanced") {
			return "Balanced"
		} else if strings.Contains(lo, "quiet") {
			return "Quiet"
		}
		return out
	}
	return "Unknown"
}

func (b *Backend) SetProfile(p string) (bool, string) {
	return b.run("profile", "set", p)
}

func (b *Backend) NextProfile() (bool, string) {
	ok, out := b.run("profile", "next")
	if ok {
		return true, b.GetProfile()
	}
	return false, out
}

func (b *Backend) ListProfiles() (bool, string) {
	return b.run("profile", "list")
}

// ─── Keyboard Brightness ─────────────────────────────────────────────────────

func (b *Backend) GetKbdBrightness() string {
	ok, out := b.run("leds", "get")
	if ok {
		lo := strings.ToLower(out)
		for _, level := range []string{"off", "low", "med", "high"} {
			if strings.Contains(lo, level) {
				return level
			}
		}
	}
	return "med"
}

func (b *Backend) SetKbdBrightness(level string) (bool, string) {
	return b.run("leds", "set", level)
}

func (b *Backend) NextKbdBrightness() (bool, string) {
	return b.run("leds", "next")
}

func (b *Backend) PrevKbdBrightness() (bool, string) {
	return b.run("leds", "prev")
}

// ─── Battery ─────────────────────────────────────────────────────────────────

func (b *Backend) GetChargeLimit() int {
	ok, out := b.run("battery", "info")
	if ok {
		// "Current battery charge limit: 70%"
		for _, field := range strings.Fields(out) {
			field = strings.TrimSuffix(field, "%")
			if v, err := strconv.Atoi(field); err == nil && v >= 20 && v <= 100 {
				return v
			}
		}
	}
	return 80
}

func (b *Backend) SetChargeLimit(pct int) (bool, string) {
	pct = clamp(pct, 20, 100)
	return b.run("battery", "limit", strconv.Itoa(pct))
}

func (b *Backend) ToggleOneShotCharge() (bool, string) {
	return b.run("battery", "oneshot")
}

// ─── Aura RGB ────────────────────────────────────────────────────────────────

type AuraState struct {
	Mode    string // e.g. "Static", "Breathe"
	R1, G1, B1 int
	R2, G2, B2 int
	Speed   string // "Low", "Med", "High"
}

func (b *Backend) GetAuraState() *AuraState {
	configs, _ := filepath.Glob("/etc/asusd/aura_*.ron")
	if len(configs) == 0 {
		return nil
	}
	data, err := os.ReadFile(configs[0])
	if err != nil {
		return nil
	}
	content := string(data)

	// Parse current_mode
	mode := parseRonField(content, "current_mode")
	if mode == "" {
		return nil
	}

	// Find the block for the current mode
	idx := strings.Index(content, mode+": (")
	if idx < 0 {
		// Try after current_mode line to skip it
		after := strings.Index(content, "builtins:")
		if after >= 0 {
			sub := content[after:]
			idx2 := strings.Index(sub, mode+": (")
			if idx2 >= 0 {
				idx = after + idx2
			}
		}
	}
	if idx < 0 {
		return nil
	}

	// Extract the block for this mode (find matching closing paren)
	block := content[idx:]
	depth := 0
	end := -1
	for i, ch := range block {
		if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
	}
	if end < 0 {
		return nil
	}
	block = block[:end]

	// Parse colour1 and colour2 blocks
	r1, g1, b1 := parseRonColour(block, "colour1")
	r2, g2, b2 := parseRonColour(block, "colour2")
	speed := parseRonField(block, "speed")

	return &AuraState{
		Mode: mode,
		R1: r1, G1: g1, B1: b1,
		R2: r2, G2: g2, B2: b2,
		Speed: speed,
	}
}

func parseRonField(s, field string) string {
	prefix := field + ": "
	idx := strings.Index(s, prefix)
	if idx < 0 {
		return ""
	}
	rest := s[idx+len(prefix):]
	end := strings.IndexAny(rest, ",\n)")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:end])
}

func parseRonColour(block, name string) (int, int, int) {
	idx := strings.Index(block, name+": (")
	if idx < 0 {
		return 0, 0, 0
	}
	sub := block[idx:]
	end := strings.Index(sub, "),")
	if end < 0 {
		end = strings.Index(sub[1:], ")")
		if end >= 0 {
			end += 1
		}
	}
	if end < 0 {
		return 0, 0, 0
	}
	sub = sub[:end]
	r, _ := strconv.Atoi(parseRonField(sub, "r"))
	g, _ := strconv.Atoi(parseRonField(sub, "g"))
	b, _ := strconv.Atoi(parseRonField(sub, "b"))
	return r, g, b
}

func (b *Backend) SetAuraMode(mode, colour1, colour2, speed string) (bool, string) {
	// Convert display name to CLI subcommand: "Rainbow Cycle" → "rainbow-cycle"
	subcmd := strings.ToLower(strings.ReplaceAll(mode, " ", "-"))
	args := []string{"aura", "effect", subcmd}
	if colour1 != "" {
		args = append(args, "--colour", colour1)
	}
	if colour2 != "" {
		args = append(args, "--colour2", colour2)
	}
	if speed != "" {
		args = append(args, "--speed", speed)
	}
	if subcmd == "rainbow-wave" {
		args = append(args, "--direction", "right")
	}
	return b.run(args...)
}

func (b *Backend) NextAuraMode() (bool, string) {
	return b.run("aura", "effect", "--next-mode")
}

func (b *Backend) PrevAuraMode() (bool, string) {
	return b.run("aura", "effect", "--prev-mode")
}

// ─── Fan Curves ──────────────────────────────────────────────────────────────

func (b *Backend) GetFanCurves(profile string) (bool, string) {
	return b.run("fan-curve", "--mod-profile", profile)
}

func (b *Backend) SetFanCurve(fan, profile, data string) (bool, string) {
	args := []string{"fan-curve"}
	if profile != "" {
		args = append(args, "--mod-profile", profile)
	}
	if fan != "" {
		args = append(args, "--fan", fan)
	}
	if data != "" {
		args = append(args, "--data", data)
	}
	return b.run(args...)
}

func (b *Backend) EnableFanCurves(profile string, enable bool) (bool, string) {
	return b.run("fan-curve", "--mod-profile", profile, "--enable-fan-curves", fmt.Sprintf("%v", enable))
}

func FormatFanCurve(temps []int, speeds []int) string {
	parts := make([]string, len(temps))
	for i := range temps {
		parts[i] = fmt.Sprintf("%dc:%d%%", temps[i], speeds[i])
	}
	return strings.Join(parts, ",")
}

// ─── BIOS ────────────────────────────────────────────────────────────────────

func (b *Backend) GetPanelOverdrive() (bool, string) {
	return b.run("armoury", "get", "panel_od")
}

func (b *Backend) SetPanelOverdrive(on bool) (bool, string) {
	val := "0"
	if on {
		val = "1"
	}
	return b.run("armoury", "set", "panel_od", val)
}

func (b *Backend) GetGpuMux() (bool, string) {
	return b.run("armoury", "get", "gpu_mux_mode")
}

func (b *Backend) SetGpuMux(dedicated bool) (bool, string) {
	val := "0"
	if dedicated {
		val = "1"
	}
	return b.run("armoury", "set", "gpu_mux_mode", val)
}

// ─── Anime / Slash ───────────────────────────────────────────────────────────

func (b *Backend) SetAnimeEnable(on bool) (bool, string) {
	return b.run("anime", "--enable-display", fmt.Sprintf("%v", on))
}

func (b *Backend) SetSlashEnable(on bool) (bool, string) {
	if on {
		return b.run("slash", "--enable")
	}
	return b.run("slash", "--disable")
}

// ─── Supported ───────────────────────────────────────────────────────────────

func (b *Backend) GetSupported() (bool, string) {
	return b.run("info", "--show-supported")
}

// ─── Raw ─────────────────────────────────────────────────────────────────────

func (b *Backend) RunRaw(args string) (bool, string) {
	parts := strings.Fields(args)
	if len(parts) == 0 {
		return false, "no arguments"
	}
	return b.run(parts...)
}
