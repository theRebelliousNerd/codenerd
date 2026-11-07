package executable

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKind(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]struct {
		body []byte
		want bool
	}{
		"elf":       {[]byte{0x7F, 'E', 'L', 'F', 2, 1}, true},
		"pe":        {[]byte("MZ\x90\x00"), true},
		"macho64":   {[]byte{0xCF, 0xFA, 0xED, 0xFE, 7}, true},
		"script.sh": {[]byte("#!/bin/sh\necho hi\n"), false},
		"source.go": {[]byte("package main\n"), false},
		"empty":     {nil, false},
		"short":     {[]byte{0x7F}, false},
	}
	for name, c := range cases {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, c.body, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, got := Kind(p); got != c.want {
			t.Errorf("%s: Kind = %v, want %v", name, got, c.want)
		}
	}
	if _, ok := Kind(filepath.Join(dir, "missing")); ok {
		t.Error("a missing file was reported as an executable")
	}
}
