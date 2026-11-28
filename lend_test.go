package moonlighter

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	_ "time/tzdata"
)

func clock(t *testing.T, h, m int) time.Time {
	t.Helper()
	return time.Date(2026, 11, 28, h, m, 0, 0, time.UTC)
}

func hoursSettings(t *testing.T, hours string) Settings {
	t.Helper()
	s, err := LoadSettings(filepath.Join(t.TempDir(), "none"), func(string) (string, bool) { return "", false }, []string{"-hours=" + hours})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDecide(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	idle := Signals{Idle: 30 * time.Minute, FreeRAM: 1 << 30}
	cases := []struct {
		name  string
		hours string
		now   time.Time
		sig   Signals
		off   bool
		lend  bool
	}{
		{"idle past threshold in hours", "09:00-17:00", clock(t, 12, 0), idle, false, true},
		{"idle exactly at threshold", "09:00-17:00", clock(t, 12, 0), Signals{Idle: 10 * time.Minute}, false, true},
		{"idle just below threshold", "09:00-17:00", clock(t, 12, 0), Signals{Idle: 10*time.Minute - 1}, false, false},
		{"owner input on a lent device", "09:00-17:00", clock(t, 12, 0), Signals{Idle: time.Hour, OwnerInput: true}, false, false},
		{"idle outside hours", "09:00-17:00", clock(t, 20, 0), idle, false, false},
		{"hours end is exclusive", "09:00-17:00", clock(t, 17, 0), idle, false, false},
		{"hours start is inclusive", "09:00-17:00", clock(t, 9, 0), idle, false, true},
		{"across midnight, evening", "22:00-06:00", clock(t, 23, 30), idle, false, true},
		{"across midnight, morning", "22:00-06:00", clock(t, 5, 59), idle, false, true},
		{"across midnight, midday", "22:00-06:00", clock(t, 12, 0), idle, false, false},
		{"second window", "01:00-02:00,09:00-17:00", clock(t, 10, 0), idle, false, true},
		{"empty hours never lend", "", clock(t, 12, 0), idle, false, false},
		{"signal error", "09:00-17:00", clock(t, 12, 0), Signals{Idle: time.Hour, Err: errors.New("boom")}, false, false},
		{"enabled off", "09:00-17:00", clock(t, 12, 0), idle, true, false},
		{"after spring-forward gap", "01:00-04:00", time.Date(2026, 3, 8, 3, 30, 0, 0, ny), idle, false, true},
		{"after fall-back repeat", "01:00-02:00", time.Date(2026, 11, 1, 1, 30, 0, 0, ny).Add(time.Hour), idle, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := hoursSettings(t, c.hours)
			s.Enabled = !c.off
			lend, reason := Decide(s, c.sig, c.now)
			if lend != c.lend || reason == "" {
				t.Fatalf("lend = %v (%q), want %v with a reason", lend, reason, c.lend)
			}
		})
	}
}

func load(t *testing.T, file string, env map[string]string, args ...string) (Settings, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "moonlighter.conf")
	if file != "" {
		if err := os.WriteFile(path, []byte(file), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return LoadSettings(path, func(k string) (string, bool) { v, ok := env[k]; return v, ok }, args)
}

func TestDefaults(t *testing.T) {
	s, err := load(t, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.LeaveMode != LeaveFinish || !s.Enabled {
		t.Fatalf("defaults: mode %q enabled %v, want finish and true", s.LeaveMode, s.Enabled)
	}
	s, err = load(t, "# note\nidle_threshold = 3m\n", nil)
	if err != nil || s.LeaveMode != LeaveFinish || !s.Enabled {
		t.Fatalf("file without leave_mode or enabled: %+v, %v", s, err)
	}
}

func TestUnknownLeaveModeNamesValue(t *testing.T) {
	for name, load := range map[string]func() (Settings, error){
		"file": func() (Settings, error) { return load(t, "leave_mode=warp\n", nil) },
		"env":  func() (Settings, error) { return load(t, "", map[string]string{"MOONLIGHTER_LEAVE_MODE": "warp"}) },
		"flag": func() (Settings, error) { return load(t, "", nil, "-leave-mode=warp") },
	} {
		s, err := load()
		if err == nil || !strings.Contains(err.Error(), "warp") {
			t.Fatalf("%s: err = %v, want one naming warp", name, err)
		}
		if lend, _ := Decide(s, Signals{Idle: time.Hour}, clock(t, 23, 0)); lend {
			t.Fatalf("%s: settings from a failed load lent", name)
		}
	}
}

func TestEnabledOffReclaims(t *testing.T) {
	s, err := load(t, "enabled=false\nhours=00:00-23:59\n", nil)
	if err != nil || s.Enabled {
		t.Fatalf("enabled = %v, err %v, want false", s.Enabled, err)
	}
	if lend, _ := Decide(s, Signals{Idle: time.Hour}, clock(t, 12, 0)); lend {
		t.Fatal("lent while enabled is off")
	}
}

func TestSettingsReadAgainEachTick(t *testing.T) {
	path := filepath.Join(t.TempDir(), "moonlighter.conf")
	env := func(string) (string, bool) { return "", false }
	for _, want := range []time.Duration{3 * time.Minute, 20 * time.Minute} {
		if err := os.WriteFile(path, []byte("idle_threshold="+want.String()+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		s, err := LoadSettings(path, env, nil)
		if err != nil || s.IdleThreshold != want {
			t.Fatalf("threshold = %v, err %v, want %v", s.IdleThreshold, err, want)
		}
	}
}

func TestPrecedence(t *testing.T) {
	cases := []struct {
		name string
		file string
		env  map[string]string
		args []string
		want time.Duration
	}{
		{"default", "", nil, nil, 10 * time.Minute},
		{"file", "idle_threshold=1m", nil, nil, time.Minute},
		{"env over file", "idle_threshold=1m", map[string]string{"MOONLIGHTER_IDLE_THRESHOLD": "2m"}, nil, 2 * time.Minute},
		{"env alone", "", map[string]string{"MOONLIGHTER_IDLE_THRESHOLD": "2m"}, nil, 2 * time.Minute},
		{"flag over env and file", "idle_threshold=1m", map[string]string{"MOONLIGHTER_IDLE_THRESHOLD": "2m"}, []string{"-idle-threshold=3m"}, 3 * time.Minute},
		{"flag alone", "", nil, []string{"-idle-threshold", "3m"}, 3 * time.Minute},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, err := load(t, c.file, c.env, c.args...)
			if err != nil || s.IdleThreshold != c.want {
				t.Fatalf("threshold = %v, err %v, want %v", s.IdleThreshold, err, c.want)
			}
		})
	}
	s, err := load(t, "leave_mode=slow", map[string]string{"MOONLIGHTER_LEAVE_MODE": "now"})
	if err != nil || s.LeaveMode != LeaveNow {
		t.Fatalf("env leave mode = %q, err %v, want now", s.LeaveMode, err)
	}
}

func TestOtherSettings(t *testing.T) {
	s, err := load(t, "max_wait=0s\nhours=\n", nil)
	if err != nil || s.MaxWait != 0 || len(s.Hours) != 0 {
		t.Fatalf("max_wait 0 and empty hours: %+v, %v", s, err)
	}
	for _, bad := range []string{"hours=25:00-26:00", "hours=09:00-09:00", "hours=9am-5pm", "idle_threshold=0s", "idle_threshold=soon", "enabled=maybe", "colour=red"} {
		if _, err := load(t, bad, nil); err == nil {
			t.Errorf("%q loaded without error", bad)
		}
	}
}
