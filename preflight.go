package moonlighter

import (
	"fmt"
	"strconv"
	"strings"
)

// WSLInstallCommand is the command a Windows owner runs when WSL is absent.
const WSLInstallCommand = "wsl --install --no-distribution"

// MinWSLMemoryBytes is the smallest WSL memory cap the preflight accepts (12 GiB).
const MinWSLMemoryBytes int64 = 12 << 30

// Host is the machine facts the preflight judges; the caller gathers them on Windows.
type Host struct {
	WSLInstalled bool   // whether WSL2 is present on the machine
	HostRAMBytes int64  // the host's total RAM in bytes
	WSLConfig    string // text of %USERPROFILE%\.wslconfig, empty when absent
}

// PreflightError says why a machine cannot host the distro yet; Command is
// set only when running one command would fix the refusal.
type PreflightError struct {
	Reason  string
	Command string
}

// Error includes the command to run, when the refusal names one.
func (e *PreflightError) Error() string {
	if e.Command == "" {
		return "preflight: " + e.Reason
	}
	return fmt.Sprintf("preflight: %s (run: %s)", e.Reason, e.Command)
}

// Preflight refuses with the install command when WSL is absent, and without
// one when the WSL memory cap is below MinWSLMemoryBytes; else it returns nil.
func Preflight(h Host) error {
	if !h.WSLInstalled {
		return &PreflightError{Reason: "WSL2 is not installed on this machine", Command: WSLInstallCommand}
	}
	if cap := WSLMemoryCap(h.WSLConfig, h.HostRAMBytes); cap < MinWSLMemoryBytes {
		return &PreflightError{Reason: fmt.Sprintf(
			"WSL memory cap is %d GiB, below the %d GiB minimum", cap>>30, MinWSLMemoryBytes>>30)}
	}
	return nil
}

// WSLMemoryCap returns the memory cap WSL would apply for the given
// .wslconfig text: the memory= value in the [wsl2] section, or, when that key
// is absent or unparseable, half of the host RAM, which is WSL's default
// (its GB unit is 1024 based, and it ignores an invalid value).
func WSLMemoryCap(config string, hostRAM int64) int64 {
	const wsl2 = "wsl2"
	found, mem, section := false, int64(0), ""
	for _, line := range strings.Split(config, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		if section != wsl2 {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok || strings.ToLower(strings.TrimSpace(key)) != "memory" {
			continue
		}
		if n, ok := parseWSLBytes(val); ok {
			mem, found = n, true
		}
	}
	if !found {
		return hostRAM / 2
	}
	return mem
}

// parseWSLBytes parses a .wslconfig size such as "12GB" (1024-based units, any case).
func parseWSLBytes(v string) (int64, bool) {
	v = strings.ToUpper(strings.TrimSpace(v))
	for _, u := range []struct {
		suffix string
		mult   int64
	}{{"TB", 1 << 40}, {"GB", 1 << 30}, {"MB", 1 << 20}, {"KB", 1 << 10}, {"B", 1}} {
		if !strings.HasSuffix(v, u.suffix) {
			continue
		}
		n, err := strconv.ParseInt(strings.TrimSpace(strings.TrimSuffix(v, u.suffix)), 10, 64)
		if err != nil || n < 0 {
			return 0, false
		}
		return n * u.mult, true
	}
	return 0, false
}
