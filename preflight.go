package moonlighter

import (
	"math"
	"strconv"
	"strings"
)

// WSLInstallCommand is the one command a Windows owner runs to get WSL2, without a distribution.
const WSLInstallCommand = "wsl --install --no-distribution"

// MinWSLMemoryBytes is the smallest WSL memory cap the machine may have: 12 GiB.
const MinWSLMemoryBytes = int64(12) << 30

// Host is what the check needs to know about the Windows machine; the caller gathers it there,
// so the check never touches the machine and builds and runs on every OS.
type Host struct {
	// WSLInstalled reports whether WSL2 is present.
	WSLInstalled bool
	// HostRAMBytes is the host's RAM in bytes.
	HostRAMBytes int64
	// WSLConfig is the text of %USERPROFILE%\.wslconfig, if any.
	WSLConfig string
}

// PreflightError says why the machine cannot host the distro yet, and, when WSL is missing, the one command.
type PreflightError struct {
	Reason  string
	Command string
}

func (e *PreflightError) Error() string {
	if e.Command == "" {
		return e.Reason
	}
	return e.Reason + "; install it with: " + e.Command
}

// Preflight reports whether the machine can host the moonlighter-managed WSL2 distro: WSL2
// must be installed and the WSL memory cap must be at least MinWSLMemoryBytes.
func Preflight(h Host) error {
	if !h.WSLInstalled {
		return &PreflightError{
			Reason:  "WSL2 is not installed",
			Command: WSLInstallCommand,
		}
	}
	if WSLMemoryCap(h.WSLConfig, h.HostRAMBytes) < MinWSLMemoryBytes {
		return &PreflightError{Reason: "the WSL memory cap is below 12 GiB"}
	}
	return nil
}

// WSLMemoryCap returns the memory cap WSL2 would apply to the distro: the memory= value of the
// [wsl2] section of the given .wslconfig text, or, when the key is absent, unparseable, or too
// large for a byte count, half of the host RAM — WSL ignores such a line and applies its default.
func WSLMemoryCap(config string, hostRAM int64) int64 {
	def, in := hostRAM/2, false
	capBytes := def
	for _, line := range strings.Split(config, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			in = strings.EqualFold(strings.Trim(line, "[]"), "wsl2")
			continue
		}
		if !in {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found || !strings.EqualFold(strings.TrimSpace(key), "memory") {
			continue
		}
		if n, ok := parseWSLBytes(value); ok {
			capBytes = n
		}
	}
	return capBytes
}

// parseWSLBytes parses a .wslconfig byte size such as "12GB"; the unit suffixes are 1024 based.
func parseWSLBytes(value string) (int64, bool) {
	value = strings.ToUpper(strings.TrimSpace(value))
	for _, u := range []struct {
		suffix string
		mult   int64
	}{{"TB", 1 << 40}, {"GB", 1 << 30}, {"MB", 1 << 20}, {"KB", 1 << 10}, {"B", 1}} {
		if !strings.HasSuffix(value, u.suffix) {
			continue
		}
		n, err := strconv.ParseInt(strings.TrimSuffix(value, u.suffix), 10, 64)
		if err != nil || n < 0 || n > math.MaxInt64/u.mult {
			return 0, false
		}
		return n * u.mult, true
	}
	return 0, false
}
