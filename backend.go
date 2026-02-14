package main

import (
	"fmt"
	"os/exec"
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

func (b *Backend) SetChargeLimit(pct int) (bool, string) {
	pct = clamp(pct, 20, 100)
	return b.run("battery", "limit", strconv.Itoa(pct))
}

func (b *Backend) ToggleOneShotCharge() (bool, string) {
	return b.run("battery", "oneshot")
}

// ─── Aura RGB ────────────────────────────────────────────────────────────────

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
