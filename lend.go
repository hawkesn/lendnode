package moonlighter

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type LeaveMode string

const LeaveFinish, LeaveSlow, LeaveNow = LeaveMode("finish"), LeaveMode("slow"), LeaveMode("now")

type Window struct{ Start, End int }

type Settings struct {
	Enabled                bool
	Hours                  []Window
	IdleThreshold, MaxWait time.Duration
	LeaveMode              LeaveMode
}

type Signals struct {
	Idle       time.Duration
	OwnerInput bool
	FreeRAM    uint64
	Err        error
}

var keys = []string{"enabled", "hours", "idle_threshold", "leave_mode", "max_wait"}

func LoadSettings(path string, lookup func(string) (string, bool), args []string) (Settings, error) {
	s := Settings{Enabled: true, Hours: []Window{{22 * 60, 6 * 60}}, IdleThreshold: 10 * time.Minute, MaxWait: 5 * time.Minute, LeaveMode: LeaveFinish}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return Settings{}, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line == "" || line[0] == '#' {
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		if err := s.set(strings.TrimSpace(k), strings.TrimSpace(v)); err != nil {
			return Settings{}, err
		}
	}
	fs := flag.NewFlagSet("moonlighter", flag.ContinueOnError)
	for _, key := range keys {
		if v, ok := lookup("MOONLIGHTER_" + strings.ToUpper(key)); ok {
			if err := s.set(key, v); err != nil {
				return Settings{}, err
			}
		}
		fs.Func(strings.ReplaceAll(key, "_", "-"), key, func(v string) error { return s.set(key, v) })
	}
	if err := fs.Parse(args); err != nil {
		return Settings{}, err
	}
	return s, nil
}

func (s *Settings) set(key, value string) (err error) {
	switch key {
	case "enabled":
		s.Enabled, err = strconv.ParseBool(value)
	case "hours":
		s.Hours = nil
		if value != "" {
			for _, p := range strings.Split(value, ",") {
				a, b, _ := strings.Cut(p, "-")
				st, e1 := time.Parse("15:04", a)
				en, e2 := time.Parse("15:04", b)
				if e1 != nil || e2 != nil || st.Hour()*60+st.Minute() == en.Hour()*60+en.Minute() {
					return fmt.Errorf("invalid hours %q", value)
				}
				s.Hours = append(s.Hours, Window{st.Hour()*60 + st.Minute(), en.Hour()*60 + en.Minute()})
			}
		}
	case "idle_threshold":
		if s.IdleThreshold, err = time.ParseDuration(value); err == nil && s.IdleThreshold <= 0 {
			err = fmt.Errorf("invalid idle_threshold %q", value)
		}
	case "max_wait":
		s.MaxWait, err = time.ParseDuration(value)
	case "leave_mode":
		if m := LeaveMode(value); m == LeaveFinish || m == LeaveSlow || m == LeaveNow {
			s.LeaveMode = m
		} else {
			err = fmt.Errorf("unknown leave mode %q", value)
		}
	default:
		err = fmt.Errorf("unknown setting %q", key)
	}
	return err
}

func Decide(s Settings, sig Signals, now time.Time) (bool, string) {
	if !s.Enabled {
		return false, "reclaim: lending is disabled"
	}
	if sig.Err != nil {
		return false, "reclaim: signal error: " + sig.Err.Error()
	}
	if sig.OwnerInput {
		return false, "reclaim: owner input on the device"
	}
	m := now.Hour()*60 + now.Minute()
	for _, w := range s.Hours {
		if m >= w.Start && (m < w.End || w.Start > w.End) || w.Start > w.End && m < w.End {
			if sig.Idle < s.IdleThreshold {
				return false, "reclaim: device not idle long enough"
			}
			return true, "lend: device idle within allowed hours"
		}
	}
	return false, "reclaim: outside allowed hours"
}
