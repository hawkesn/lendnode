package moonlighter

import (
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestIdleSinceHandlesTickWrap(t *testing.T) {
	cases := []struct {
		name       string
		tick, last uint32
		want       time.Duration
	}{
		{"plain", 5000, 2000, 3 * time.Second},
		{"same tick", 7, 7, 0},
		{"tick count wrapped after the last input", 100, 0xFFFFFF00, 356 * time.Millisecond},
	}
	for _, c := range cases {
		if got := IdleSince(c.tick, c.last); got != c.want {
			t.Errorf("%s: IdleSince(%d, %d) = %v, want %v", c.name, c.tick, c.last, got, c.want)
		}
	}
}

func TestFullScreenGameStates(t *testing.T) {
	// SHQueryUserNotificationState: 2 busy, 3 Direct3D full screen, 4 presentation mode.
	for state, want := range map[uint32]bool{0: false, 1: false, 2: true, 3: true, 4: true, 5: false, 6: false, 7: false} {
		if got := FullScreenGame(state); got != want {
			t.Errorf("FullScreenGame(%d) = %v, want %v", state, got, want)
		}
	}
}

func TestFormatSignalsPrintsEachSignal(t *testing.T) {
	out := FormatSignals(Signals{Idle: 90 * time.Second, OwnerInput: true, FreeRAM: 3 << 30})
	for _, want := range []string{"idle: 1m30s\n", "game: true\n", "free_ram: 3072 MiB\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q lacks %q", out, want)
		}
	}
	if strings.Contains(out, "error") {
		t.Errorf("output %q names an error though there was none", out)
	}
	if out := FormatSignals(Signals{Err: errors.New("no session")}); !strings.Contains(out, "error: no session\n") {
		t.Errorf("output %q lacks the error line", out)
	}
}

func TestReadSignalsFailsClosedOffWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the Windows reading is proven on orion-mk4, not here")
	}
	sig := ReadSignals()
	if sig.Err == nil {
		t.Fatalf("ReadSignals on %s returned no error: %+v", runtime.GOOS, sig)
	}
	s := Settings{Enabled: true, Hours: []Window{{0, 23*60 + 59}}, IdleThreshold: time.Minute}
	if lend, _ := Decide(s, sig, time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC)); lend {
		t.Fatal("a signal read that failed still lent the node")
	}
}

func TestControllerGameDoesNotLendOnIdleAlone(t *testing.T) {
	s := Settings{Enabled: true, Hours: []Window{{0, 23*60 + 59}}, IdleThreshold: time.Minute}
	now := time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC)
	if lend, _ := Decide(s, Signals{Idle: time.Hour, OwnerInput: true}, now); lend {
		t.Fatal("a full-screen game with an hour of idle still lent")
	}
	if lend, _ := Decide(s, Signals{Idle: time.Hour}, now); !lend {
		t.Fatal("an idle machine with no game did not lend")
	}
}
