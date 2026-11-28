package moonlighter

import (
	"errors"
	"testing"
)

func TestPreflightMemoryRefusalHasNoCommand(t *testing.T) {
	err := Preflight(Host{WSLInstalled: true, HostRAMBytes: 8 * gib})
	var pe *PreflightError
	if !errors.As(err, &pe) {
		t.Fatalf("want *PreflightError, got %v", err)
	}
	if pe.Command != "" {
		t.Fatalf("memory refusal must not carry a command, got %q", pe.Command)
	}
}

func TestPreflightKeyBeforeAnySectionIgnored(t *testing.T) {
	// A memory= line before any section header is not in [wsl2], so the default
	// of half the host RAM applies.
	cfg := "memory=2GB\n[wsl2]\nprocessors=4\n"
	if err := Preflight(Host{WSLInstalled: true, HostRAMBytes: 32 * gib, WSLConfig: cfg}); err != nil {
		t.Fatalf("want pass, got %v", err)
	}
}

func TestWSLMemoryCapUnits(t *testing.T) {
	cases := []struct {
		name string
		cfg  string
		ram  int64
		want int64
	}{
		{"tb", "[wsl2]\nmemory=4194304TB\n", 8 * gib, int64(4194304) << 40},
		{"gb", "[wsl2]\nmemory=6GB\n", 64 * gib, 6 << 30},
		{"mb", "[wsl2]\nmemory=6144MB\n", 64 * gib, 6144 << 20},
		{"kb", "[wsl2]\nmemory=6291456KB\n", 64 * gib, 6291456 << 10},
		{"b", "[wsl2]\nmemory=7000000000B\n", 64 * gib, 7000000000},
		{"lower case unit", "[wsl2]\nmemory=6gb\n", 64 * gib, 6 << 30},
		{"huge value falls back to default", "[wsl2]\nmemory=9000000000GB\n", 64 * gib, 32 * gib},
		{"unparsable number falls back", "[wsl2]\nmemory=12GBx\n", 64 * gib, 32 * gib},
		{"negative falls back", "[wsl2]\nmemory=-4GB\n", 64 * gib, 32 * gib},
		{"missing number falls back", "[wsl2]\nmemory=GB\n", 64 * gib, 32 * gib},
	}
	for _, c := range cases {
		if got := WSLMemoryCap(c.cfg, c.ram); got != c.want {
			t.Errorf("%s: cap = %d, want %d", c.name, got, c.want)
		}
	}
}
