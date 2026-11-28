package moonlighter

import (
	"errors"
	"strings"
	"testing"
)

const gib = int64(1) << 30

func TestPreflightWSLAbsent(t *testing.T) {
	err := Preflight(Host{WSLInstalled: false, HostRAMBytes: 64 * gib})
	var pe *PreflightError
	if !errors.As(err, &pe) {
		t.Fatalf("want *PreflightError, got %v", err)
	}
	if pe.Command != "wsl --install --no-distribution" {
		t.Fatalf("command = %q", pe.Command)
	}
	if !strings.Contains(err.Error(), "wsl --install --no-distribution") {
		t.Fatalf("message lacks command: %q", err.Error())
	}
}

func TestPreflightMemoryCap(t *testing.T) {
	cases := []struct {
		name string
		cfg  string
		ram  int64
		ok   bool
	}{
		{"explicit 12GB passes", "[wsl2]\nmemory=12GB\n", 64 * gib, true},
		{"explicit 11GB refuses", "[wsl2]\nmemory=11GB\n", 64 * gib, false},
		{"explicit 8GB refuses", "[wsl2]\nmemory=8GB\n", 64 * gib, false},
		{"megabytes", "[wsl2]\nmemory=12288MB\n", 64 * gib, true},
		{"default is half of host, 32 passes", "", 32 * gib, true},
		{"default is half of host, 16 refuses", "", 16 * gib, false},
		{"other section ignored", "[experimental]\nmemory=2GB\n[wsl2]\nprocessors=4\n", 32 * gib, true},
		{"key case, spaces and comment", "[WSL2]\n; memory=1GB\n Memory = 16GB \n", 8 * gib, true},
		{"unparseable falls back to default", "[wsl2]\nmemory=lots\n", 16 * gib, false},
	}
	for _, c := range cases {
		err := Preflight(Host{WSLInstalled: true, HostRAMBytes: c.ram, WSLConfig: c.cfg})
		if c.ok && err != nil {
			t.Errorf("%s: unexpected error %v", c.name, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s: want refusal", c.name)
		}
	}
}

func TestPreflightPass(t *testing.T) {
	if err := Preflight(Host{WSLInstalled: true, HostRAMBytes: 64 * gib}); err != nil {
		t.Fatal(err)
	}
}
