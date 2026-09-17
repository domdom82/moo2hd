package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/domdom82/moo2hd/internal/lbx"
)

// playSound writes r.Data to a temp file and starts a platform-appropriate audio player.
// Returns the running Cmd and the temp file path (caller must os.Remove on cleanup).
func playSound(r *lbx.Record) (*exec.Cmd, string, error) {
	ext := strings.ToLower(r.Type.String())
	tmp, err := os.CreateTemp("", "moo2hd-*."+ext)
	if err != nil {
		return nil, "", fmt.Errorf("creating temp file: %w", err)
	}
	if _, err := tmp.Write(r.Data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, "", fmt.Errorf("writing temp file: %w", err)
	}
	tmp.Close()

	cmd, err := playerCmd(r.Type, tmp.Name())
	if err != nil {
		os.Remove(tmp.Name())
		return nil, "", err
	}
	if err := cmd.Start(); err != nil {
		os.Remove(tmp.Name())
		return nil, "", fmt.Errorf("starting player: %w", err)
	}
	return cmd, tmp.Name(), nil
}

func playerCmd(t lbx.RecordType, path string) (*exec.Cmd, error) {
	switch t {
	case lbx.RecordWAV:
		return wavCmd(path), nil
	case lbx.RecordVOC:
		return vocCmd(path), nil
	case lbx.RecordXMI:
		return xmiCmd(path), nil
	default:
		return nil, fmt.Errorf("no player for record type %s", t)
	}
}

func wavCmd(path string) *exec.Cmd {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("afplay", path)
	case "linux":
		if _, err := exec.LookPath("aplay"); err == nil {
			return exec.Command("aplay", path)
		}
	}
	return exec.Command("ffplay", "-nodisp", "-autoexit", path)
}

func vocCmd(path string) *exec.Cmd {
	// SoX can play VOC directly on most systems.
	if _, err := exec.LookPath("sox"); err == nil {
		return exec.Command("sox", path, "-d")
	}
	return exec.Command("ffplay", "-nodisp", "-autoexit", path)
}

func xmiCmd(path string) *exec.Cmd {
	if _, err := exec.LookPath("timidity"); err == nil {
		return exec.Command("timidity", path)
	}
	if _, err := exec.LookPath("fluidsynth"); err == nil {
		return exec.Command("fluidsynth", "-a", "alsa", "-i", path)
	}
	return exec.Command("ffplay", "-nodisp", "-autoexit", path)
}
