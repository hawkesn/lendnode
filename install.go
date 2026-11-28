package moonlighter

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
)

// MergeWSLConfig sets networkingMode=mirrored in the [wsl2] section of a .wslconfig text and keeps every other line as it was.
func MergeWSLConfig(config string) string {
	lines := strings.Split(config, "\n")
	in, header, replaced := false, -1, false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			in = strings.EqualFold(trimmed, "[wsl2]")
		}
		if in && header < 0 {
			header = i
		}
		if in && replaced && !strings.HasPrefix(trimmed, "[") &&
			!strings.HasPrefix(trimmed, ";") && !strings.HasPrefix(trimmed, "#") {
			// In an ini the last occurrence wins, so a later
			// networkingMode line must be rewritten too or the mode
			// could still come up non-mirrored.
			dup, _, _ := strings.Cut(trimmed, "=")
			if strings.EqualFold(strings.TrimSpace(dup), "networkingMode") {
				dupSet := "networkingMode=mirrored"
				if strings.HasSuffix(line, "\r") {
					dupSet += "\r"
				}
				lines[i] = dupSet
			}
		}
		if !in || replaced || strings.HasPrefix(trimmed, "[") ||
			strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, _, _ := strings.Cut(trimmed, "=")
		if !strings.EqualFold(strings.TrimSpace(key), "networkingMode") {
			continue
		}
		set := "networkingMode=mirrored"
		if strings.HasSuffix(line, "\r") {
			set += "\r"
		}
		lines[i] = set
		replaced = true
	}
	if replaced {
		return strings.Join(lines, "\n")
	}
	cr := ""
	if strings.Contains(config, "\r\n") {
		cr = "\r"
	}
	if header >= 0 {
		merged := append([]string{}, lines[:header+1]...)
		merged = append(merged, "networkingMode=mirrored"+cr)
		return strings.Join(append(merged, lines[header+1:]...), "\n")
	}
	if config != "" && !strings.HasSuffix(config, "\n") {
		config += "\n"
	}
	return config + "[wsl2]" + cr + "\nnetworkingMode=mirrored" + cr + "\n"
}

// VerifyRootfs reports whether the rootfs stream matches the pinned sha256; an empty pin is refused.
func VerifyRootfs(r io.Reader, pin string) error {
	sum := sha256.New()
	if _, err := io.Copy(sum, r); err != nil {
		return err
	}
	got := hex.EncodeToString(sum.Sum(nil))
	if pin == "" || !strings.EqualFold(got, pin) {
		return errors.New("rootfs sha256 " + got + " does not match the pinned " + pin)
	}
	return nil
}
