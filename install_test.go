package moonlighter

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestMergeWSLConfig(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"empty file", "", "[wsl2]\nnetworkingMode=mirrored\n"},
		{"keeps owner keys",
			"[wsl2]\nmemory=24GB\nprocessors=16\nswap=4GB\n",
			"[wsl2]\nnetworkingMode=mirrored\nmemory=24GB\nprocessors=16\nswap=4GB\n"},
		{"replaces other mode",
			"[wsl2]\nmemory=24GB\nnetworkingMode=NAT\n[experimental]\nautoMemoryReclaim=gradual\n",
			"[wsl2]\nmemory=24GB\nnetworkingMode=mirrored\n[experimental]\nautoMemoryReclaim=gradual\n"},
		{"no wsl2 section appends one",
			"[experimental]\nsparseVhd=true",
			"[experimental]\nsparseVhd=true\n[wsl2]\nnetworkingMode=mirrored\n"},
		{"key in another section is not the mode",
			"[experimental]\nnetworkingMode=nat\n",
			"[experimental]\nnetworkingMode=nat\n[wsl2]\nnetworkingMode=mirrored\n"},
		{"section and key case, comment kept",
			"[WSL2]\n; networkingMode=nat\nNetworkingMode = nat\n",
			"[WSL2]\n; networkingMode=nat\nnetworkingMode=mirrored\n"},
		{"crlf stays crlf",
			"[wsl2]\r\nmemory=24GB\r\n",
			"[wsl2]\r\nnetworkingMode=mirrored\r\nmemory=24GB\r\n"},
	}
	for _, c := range cases {
		got := MergeWSLConfig(c.in)
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
		if again := MergeWSLConfig(got); again != got {
			t.Errorf("%s: second merge changed the text: %q", c.name, again)
		}
	}
}

func TestVerifyRootfs(t *testing.T) {
	sum := sha256.Sum256([]byte("rootfs"))
	pin := hex.EncodeToString(sum[:])
	if err := VerifyRootfs(strings.NewReader("rootfs"), pin); err != nil {
		t.Fatalf("matching pin: %v", err)
	}
	if err := VerifyRootfs(strings.NewReader("rootfs"), strings.ToUpper(pin)); err != nil {
		t.Fatalf("upper-case pin: %v", err)
	}
	if err := VerifyRootfs(strings.NewReader("rootfs!"), pin); err == nil {
		t.Fatal("changed bytes must fail")
	}
	if err := VerifyRootfs(strings.NewReader("rootfs"), ""); err == nil {
		t.Fatal("an empty pin must fail")
	}
}
